package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/xthexder/go-jack"
	"github.com/youpy/go-wav"
)

const (
	CAPTURE_WAVE_FILE  = "../Record.wav"
	PLAYBACK_WAVE_FILE = "../Sample.wav"
)

func main() {
	client, _ := jack.ClientOpen("AcousticLink", jack.NoStartServer)
	if client == nil {
		fmt.Println("Could not connect to jack server.")
		return
	}
	defer client.Close()

	inPort := client.PortRegister("input", jack.DEFAULT_AUDIO_TYPE, jack.PortIsInput, 0)
	outPort := client.PortRegister("output", jack.DEFAULT_AUDIO_TYPE, jack.PortIsOutput, 0)

	systemInPort := client.GetPortByName("system:capture_1")
	systemOutPort := client.GetPortByName("system:playback_1")

	inputChannel := make(chan jack.AudioSample, 1024)
	outputChannel := make(chan jack.AudioSample, 1024)

	process := func(nframes uint32) int {
		inBuffer := inPort.GetBuffer(nframes)
		outBuffer := outPort.GetBuffer(nframes)

		for _, sample := range inBuffer {
			inputChannel <- sample
		}

		for i := range outBuffer {
			select {
			case sample := <-outputChannel:
				outBuffer[i] = sample
			default:
				outBuffer[i] = 0.0
			}
		}

		return 0
	}

	go func() {
		file, err := os.Create(CAPTURE_WAVE_FILE)
		if err != nil {
			fmt.Println("Could not create WAV file:", err)
			return
		}
		defer file.Close()

		writer := wav.NewWriter(file, 0, 1, client.GetSampleRate(), 32)

		for {
			sample := <-inputChannel

			samples := make([]wav.Sample, 1)
			samples[0].Values[0] = int(sample * math.MaxInt32)

			err = writer.WriteSamples(samples)
			if err != nil {
				fmt.Println("Error writing samples:", err)
				return
			}
		}
	}()

	go func() {
		file, err := os.Open(PLAYBACK_WAVE_FILE)
		if err != nil {
			fmt.Println("Could not open WAV file:", err)
			return
		}
		defer file.Close()

		reader := wav.NewReader(file)

		for {
			samples, err := reader.ReadSamples(1)
			if err == io.EOF {
				break
			}

			outputChannel <- jack.AudioSample(reader.FloatValue(samples[0], 0))
		}
	}()

	if code := client.SetProcessCallback(process); code != 0 {
		fmt.Println("Failed to set process callback.")
		return
	}

	if code := client.Activate(); code != 0 {
		fmt.Println("Failed to activate client.")
		return
	}

	client.ConnectPorts(systemInPort, inPort)
	client.ConnectPorts(outPort, systemOutPort)

	fmt.Println("Press enter or return to quit...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}
