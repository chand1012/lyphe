import { AgentAccess } from "~/components/agent-access";
import { toast } from "sonner";
import { useState } from "react";
import { useTheme } from "next-themes";
import { Link, useNavigate } from "react-router";
import { Button } from "~/components/ui/button";
import { Label } from "~/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "~/components/ui/select";
import { Separator } from "~/components/ui/separator";
import { Switch } from "~/components/ui/switch";
import { applyAppearance } from "~/lib/appearance";
import { EmptyState } from "~/components/empty-state";
import { useAuth } from "~/lib/auth";
import { aiStatus, preferences, flushAll, updatePreferences, useDb } from "~/lib/data";
import { pb } from "~/lib/pocketbase";

const sections = ["General", "Appearance", "Editor", "AI", "Storage", "Agent access", "Account"] as const;
type Section = (typeof sections)[number];

export default function Settings() {
  useDb();
  const [section, setSection] = useState<Section>("Appearance");
  const { theme, setTheme } = useTheme();
  const [font, setFont] = useState(() => typeof window === "undefined" ? "lora" : localStorage.getItem("lyphe-font") ?? "lora");
  const [compact, setCompact] = useState(() => typeof window !== "undefined" && localStorage.getItem("lyphe-compact") === "true");
  const user = useAuth();
  const navigate = useNavigate();

  return (
    <div className="flex flex-col gap-3">
      <h1 className="text-3xl font-heading">Settings</h1>

      <div className="flex flex-col gap-6 sm:flex-row">
        <nav className="flex gap-1 overflow-x-auto sm:w-40 sm:shrink-0 sm:flex-col">
          {sections.map((name) => (
            <button
              key={name}
              onClick={() => setSection(name)}
              className={`shrink-0 rounded-md px-2 py-1 text-left text-sm ${section === name ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground"}`}
            >
              {name}
            </button>
          ))}
        </nav>

        <div className="flex min-w-0 flex-1 flex-col gap-3">
          {section === "Appearance" && (
            <div className="flex flex-col gap-3">
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor="theme">Theme</Label>
                <Select value={theme} onValueChange={setTheme}>
                  <SelectTrigger id="theme" size="sm" className="w-40">
                    <SelectValue placeholder="System" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="system">System</SelectItem>
                    <SelectItem value="dark">Dark</SelectItem>
                    <SelectItem value="light">Light</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor="content-font">Document font</Label>
                <Select value={font} onValueChange={(value) => { setFont(value); localStorage.setItem("lyphe-font", value); applyAppearance(); }}>
                  <SelectTrigger id="content-font" size="sm" className="w-40"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="lora">Lora</SelectItem>
                    <SelectItem value="geist">Geist</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor="compact">Compact mode</Label>
                <Switch id="compact" checked={compact} onCheckedChange={(value) => { setCompact(value); localStorage.setItem("lyphe-compact", String(value)); applyAppearance(); }} />
              </div>
            </div>
          )}

          {section === "Account" && (
            <div className="flex flex-col gap-3">
              <p className="text-sm">{user?.name ?? "Signed out"}</p>
              <p className="text-sm text-muted-foreground">{user?.email ?? "No email"}</p>
              <Button variant="outline" size="sm" onClick={async () => { try { await flushAll(); pb.authStore.clear(); navigate("/login"); } catch { toast.error("Save your pending changes before signing out"); } }}>
                Sign out
              </Button>
            </div>
          )}

          {section === "General" && <div className="flex items-center justify-between gap-3"><Label htmlFor="timezone">Timezone</Label><select id="timezone" className="rounded-md border bg-background px-2 py-1 text-sm" value={preferences?.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone} onChange={(event) => { void updatePreferences({timezone: event.target.value}); }}>{Array.from(new Set([preferences?.timezone, Intl.DateTimeFormat().resolvedOptions().timeZone, "UTC", ...Intl.supportedValuesOf("timeZone")])).filter(Boolean).map((zone) => <option key={zone} value={zone}>{zone}</option>)}</select></div>}
          {section === "Editor" && <EmptyState title="Editor settings" hint="The document editor uses Plate's defaults in V1." />}
          {section === "AI" && <div className="flex flex-col gap-4">
            <p className="text-xs text-muted-foreground">Local model: {aiStatus?.completion.state ?? "unavailable"} · Transcription: {aiStatus?.transcription.available ? "ready" : "unavailable"}</p>
            {[{key: "enable_ai", label: "Enable local language model"}, {key: "autocomplete", label: "Writing autocomplete"}, {key: "enable_transcription", label: "Transcribe finished recordings"}, {key: "cleanup", label: "Light transcript cleanup"}].map(({key, label}) => <div key={key} className="flex items-center justify-between"><Label htmlFor={key}>{label}</Label><Switch id={key} checked={!!preferences?.[key]} onCheckedChange={(value) => { void updatePreferences({[key]: value}); }} /></div>)}
            <p className="text-xs text-muted-foreground">Audio and writing are processed on your server. Original transcripts are retained.</p>
          </div>}
          {section === "Agent access" && user && <AgentAccess key={user.id} />}
          {section === "Storage" && <EmptyState title="Storage settings" hint="Attachments are stored privately on your server. The default limit is 25 MiB per file." />}
        </div>
      </div>

      <Separator />
      <p className="text-xs text-muted-foreground">
        <Link to="/journal">Back to journal</Link>
      </p>
    </div>
  );
}
