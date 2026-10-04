import { useEffect, useRef, useState } from "react";
import { MicIcon } from "lucide-react";
import { toast } from "sonner";
import { Button } from "~/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "~/components/ui/dialog";
import { clock } from "~/lib/format";

function AudioVisualizer({ stream }: { stream: MediaStream }) {
  const canvas = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const element = canvas.current;
    if (!element || !window.AudioContext) return;
    const audio = new AudioContext();
    const source = audio.createMediaStreamSource(stream);
    const analyser = audio.createAnalyser();
    analyser.fftSize = 256;
    analyser.smoothingTimeConstant = 0.72;
    source.connect(analyser);
    const samples = new Uint8Array(analyser.frequencyBinCount);
    const context = element.getContext("2d");
    const color = getComputedStyle(element).color;
    let frame = 0;

    const draw = () => {
      if (!context) return;
      const { width, height } = element.getBoundingClientRect();
      const scale = window.devicePixelRatio || 1;
      const pixelWidth = Math.max(1, Math.round(width * scale));
      const pixelHeight = Math.max(1, Math.round(height * scale));
      if (element.width !== pixelWidth || element.height !== pixelHeight) {
        element.width = pixelWidth;
        element.height = pixelHeight;
      }
      context.clearRect(0, 0, pixelWidth, pixelHeight);
      analyser.getByteFrequencyData(samples);
      const bars = 36;
      const step = pixelWidth / bars;
      context.fillStyle = color;
      for (let i = 0; i < bars; i++) {
        // The low and middle frequencies carry most speech energy.
        const from = Math.floor((i / bars) * samples.length * 0.7);
        const to = Math.max(from + 1, Math.floor(((i + 1) / bars) * samples.length * 0.7));
        let peak = 0;
        for (let j = from; j < to; j++) peak = Math.max(peak, samples[j]);
        const barHeight = Math.max(3 * scale, (peak / 255) * pixelHeight * 0.9);
        const barWidth = Math.max(2 * scale, step * 0.55);
        context.fillRect(i * step + (step - barWidth) / 2, (pixelHeight - barHeight) / 2, barWidth, barHeight);
      }
      frame = requestAnimationFrame(draw);
    };
    frame = requestAnimationFrame(draw);
    return () => {
      cancelAnimationFrame(frame);
      source.disconnect();
      analyser.disconnect();
      void audio.close();
    };
  }, [stream]);

  return <canvas ref={canvas} aria-label="Live microphone audio levels" role="img" className="h-24 w-full text-primary" />;
}

export function VoiceRecorder({ onFinish }: { onFinish: (blob: Blob, duration: number) => void }) {
  const [open, setOpen] = useState(false);
  const [starting, setStarting] = useState(false);
  const [stream, setStream] = useState<MediaStream | null>(null);
  const [seconds, setSeconds] = useState(0);
  const recorder = useRef<MediaRecorder | null>(null);
  const elapsed = useRef(0);
  const session = useRef<{ save: boolean; duration: number } | null>(null);
  const request = useRef(0);

  useEffect(() => {
    if (!stream) return;
    const timer = window.setInterval(() => { elapsed.current += 1; setSeconds(elapsed.current); }, 1000);
    return () => window.clearInterval(timer);
  }, [stream]);

  useEffect(() => () => {
    request.current++;
    if (session.current) session.current.save = false;
    recorder.current?.stream.getTracks().forEach((track) => track.stop());
    if (recorder.current?.state === "recording") recorder.current.stop();
  }, []);

  const stop = (finish: boolean) => {
    request.current++;
    if (session.current) {
      session.current.save = finish;
      session.current.duration = elapsed.current;
    }
    if (recorder.current?.state === "recording") recorder.current.stop();
    setStream(null);
    setStarting(false);
    setOpen(false);
  };

  const start = async () => {
    if (!navigator.mediaDevices?.getUserMedia || !window.MediaRecorder) {
      toast.error("Recording is not supported in this browser");
      return;
    }
    const currentRequest = ++request.current;
    setStarting(true);
    let microphone: MediaStream | null = null;
    try {
      microphone = await navigator.mediaDevices.getUserMedia({ audio: true });
      if (currentRequest !== request.current) {
        microphone.getTracks().forEach((track) => track.stop());
        return;
      }
      const media = new MediaRecorder(microphone);
      const chunks: Blob[] = [];
      const currentSession = { save: false, duration: 0 };
      media.ondataavailable = (event) => { if (event.data.size) chunks.push(event.data); };
      media.onstop = () => {
        microphone?.getTracks().forEach((track) => track.stop());
        if (currentSession.save && chunks.length) onFinish(new Blob(chunks, { type: media.mimeType }), currentSession.duration);
        if (recorder.current === media) {
          recorder.current = null;
          session.current = null;
        }
      };
      recorder.current = media;
      session.current = currentSession;
      elapsed.current = 0;
      setSeconds(0);
      media.start();
      setStream(microphone);
    } catch {
      microphone?.getTracks().forEach((track) => track.stop());
      if (currentRequest === request.current) toast.error("Could not access the microphone");
    } finally {
      if (currentRequest === request.current) setStarting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next) stop(false); else setOpen(true); }}>
      <DialogTrigger asChild><Button variant="ghost" size="sm"><MicIcon className="size-4" />Record</Button></DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Voice note</DialogTitle>
          <DialogDescription>Finish to add the recording to this journal entry.</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col items-center gap-3 py-4">
          <div className="flex h-28 w-full items-center justify-center rounded-lg bg-muted/40 px-4">
            {stream ? <AudioVisualizer stream={stream} /> : <MicIcon className="size-8 text-muted-foreground" aria-hidden="true" />}
          </div>
          <p className="text-sm tabular-nums" aria-live="polite">
            {stream ? `Recording · ${clock(seconds)}` : starting ? "Connecting to microphone…" : "Ready to record"}
          </p>
        </div>
        <DialogFooter className="bg-transparent">
          <Button variant="ghost" onClick={() => stop(false)}>{stream ? "Discard" : "Cancel"}</Button>
          {stream ? <Button onClick={() => stop(true)}>Finish</Button> : <Button disabled={starting} onClick={() => void start()}>Start recording</Button>}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
