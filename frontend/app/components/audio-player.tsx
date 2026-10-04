import { useRef, useState } from "react";
import { PauseIcon, PlayIcon, Trash2Icon } from "lucide-react";
import { toast } from "sonner";
import { Button } from "~/components/ui/button";
import { Slider } from "~/components/ui/slider";
import { clock } from "~/lib/format";

export function AudioPlayer({ name, duration, url, onRemove }: {
  name: string;
  duration: number;
  url?: string;
  onRemove?: () => void;
}) {
  const audio = useRef<HTMLAudioElement>(null);
  const [time, setTime] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [length, setLength] = useState(duration);
  const [failed, setFailed] = useState(false);

  const toggle = async () => {
    if (!audio.current) return;
    if (playing) audio.current.pause();
    else {
      try { await audio.current.play(); }
      catch { toast.error("This audio could not be played"); }
    }
  };

  return (
    <div className="flex flex-col gap-1 py-1">
      {url && <audio ref={audio} src={url} preload="metadata"
        onLoadedMetadata={(event) => {
          const seconds = event.currentTarget.duration;
          if (Number.isFinite(seconds)) setLength(seconds);
        }}
        onError={() => { setFailed(true); setPlaying(false); }}
        onTimeUpdate={(event) => setTime(event.currentTarget.currentTime)}
        onPause={() => setPlaying(false)} onPlay={() => setPlaying(true)}
        onEnded={() => { if (audio.current) audio.current.currentTime = 0; setTime(0); setPlaying(false); }}
      />}
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="icon" disabled={!url || failed} aria-label={`${playing ? "Pause" : "Play"} ${name}`} onClick={toggle}>
          {playing ? <PauseIcon /> : <PlayIcon />}
        </Button>
        <Slider aria-label={`Seek ${name}`} value={[time]} max={length || 1} disabled={!url || failed} onValueChange={([value]) => {
          if (audio.current) audio.current.currentTime = value;
          setTime(value);
        }} className="flex-1" />
        <span className="text-xs text-muted-foreground tabular-nums">{clock(Math.floor(length))}</span>
        {onRemove && <Button variant="ghost" size="icon" className="text-muted-foreground" aria-label={`Delete audio ${name}`} onClick={onRemove}><Trash2Icon className="size-4" /></Button>}
      </div>
      <p className="text-xs text-muted-foreground">{name}{(!url || failed) && " · audio unavailable"}</p>
    </div>
  );
}
