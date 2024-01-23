const CAPTURE_WAVE_FILE: &str = "Record.wav";
const PLAYBACK_WAVE_FILE: &str = "Sample.wav";

fn main() {
    let (client, _status) =
        jack::Client::new("AcousticLink", jack::ClientOptions::NO_START_SERVER).unwrap();

    let in_port = client
        .register_port("input", jack::AudioIn::default())
        .unwrap();
    let mut out_port = client
        .register_port("output", jack::AudioOut::default())
        .unwrap();

    let in_port_name = in_port.name().unwrap();
    let out_port_name = out_port.name().unwrap();

    let sample_rate = client.sample_rate() as u32;

    let (input_sender, input_receiver) = crossbeam_channel::unbounded();
    let (output_sender, output_receiver) = crossbeam_channel::unbounded();

    let process_callback = move |_: &jack::Client, ps: &jack::ProcessScope| -> jack::Control {
        let in_port_slice = in_port.as_slice(ps);
        let out_port_slice = out_port.as_mut_slice(ps);

        for input in in_port_slice.iter() {
            input_sender.try_send(*input).unwrap();
        }
        for output in out_port_slice.iter_mut() {
            *output = output_receiver.try_recv().unwrap_or(0.0)
        }

        jack::Control::Continue
    };

    let input_thread = std::thread::spawn(move || {
        let wav_spec = hound::WavSpec {
            channels: 1,
            bits_per_sample: 32,
            sample_rate,
            sample_format: hound::SampleFormat::Float,
        };
        let mut writer = hound::WavWriter::create(CAPTURE_WAVE_FILE, wav_spec).unwrap();
        loop {
            let sample = input_receiver.recv().unwrap();
            writer.write_sample(sample).unwrap();
        }
    });

    let output_thread = std::thread::spawn(move || {
        let mut reader = hound::WavReader::open(PLAYBACK_WAVE_FILE).unwrap();
        for sample in reader.samples::<i16>() {
            const AMPLITUDE: f32 = i16::MAX as f32;
            let sample = sample.unwrap() as f32 / AMPLITUDE;
            output_sender.send(sample).unwrap();
        } 
    });

    let process = jack::ClosureProcessHandler::new(process_callback);
    let active_client = client.activate_async((), process).unwrap();

    let client = active_client.as_client();
    client
        .connect_ports_by_name("system:capture_1", &in_port_name)
        .unwrap();
    client
        .connect_ports_by_name(&out_port_name, "system:playback_1")
        .unwrap();

    input_thread.join().unwrap();
    output_thread.join().unwrap();

    active_client.deactivate().unwrap();
}
