import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "~/components/ui/button";
import { Label } from "~/components/ui/label";
import { pb } from "~/lib/pocketbase";

type Credential = { id: string; name: string; expiration: string; created: string };
const inputClass = "w-full rounded-md border bg-background px-3 py-2 text-sm";

function defaultExpiration() {
  const date = new Date();
  date.setDate(date.getDate() + 90);
  date.setMinutes(date.getMinutes() - date.getTimezoneOffset());
  return date.toISOString().slice(0, 16);
}

export function AgentAccess() {
  const [credentials, setCredentials] = useState<Credential[]>([]);
  const [name, setName] = useState("");
  const [expiration, setExpiration] = useState(defaultExpiration);
  const [secret, setSecret] = useState<{ id: string; value: string }>();
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const endpoint = new URL("/mcp", pb.baseURL).href;

  useEffect(() => {
    const controller = new AbortController();
    void pb.send<{ items: Credential[] }>("/api/mcp/tokens", { signal: controller.signal }).then((result) => {
      if (!controller.signal.aborted) setCredentials(result.items);
    }).catch(() => {
      if (!controller.signal.aborted) setError("Could not load credentials. Reopen this section to try again.");
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, []);

  async function create(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setSecret(undefined);
    try {
      const date = new Date(expiration);
      if (!name.trim() || !Number.isFinite(date.getTime()) || date <= new Date()) {
        toast.error("Enter a name and a future expiration.");
        return;
      }
      const result = await pb.send<{ credential: Credential; token: string }>("/api/mcp/tokens", {
        method: "POST", body: { name: name.trim(), expiration: date.toISOString() },
      });
      setCredentials((items) => [result.credential, ...items]);
      setSecret({ id: result.credential.id, value: result.token });
      setName("");
    } catch {
      toast.error("Could not create credential. Check the list before retrying if the connection failed.");
    } finally {
      setBusy(false);
    }
  }

  async function revoke(id: string) {
    setBusy(true);
    try {
      await pb.send(`/api/mcp/tokens/${id}`, { method: "DELETE" });
      setCredentials((items) => items.filter((item) => item.id !== id));
      if (secret?.id === id) setSecret(undefined);
      toast.success("Credential revoked.");
    } catch {
      toast.error("Could not revoke credential.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <h2 className="font-medium">Connect an agent</h2>
        <p className="text-sm text-muted-foreground">Agents can manage your tasks, habits, and goals, including their documents. Journals are human written: agents can read them, but cannot create, edit, delete, restore, or duplicate them.</p>
        <p className="text-sm text-muted-foreground">File transfer, transcription, account settings, and credential management are unavailable to agents.</p>
        <Label htmlFor="mcp-endpoint">MCP endpoint</Label>
        <input id="mcp-endpoint" className={inputClass} value={endpoint} readOnly />
        <p className="text-sm text-muted-foreground">Use Streamable HTTP and an Authorization header with Bearer followed by your credential. Use HTTPS when connecting remotely.</p>
      </div>
      <form onSubmit={(event) => { void create(event); }} className="flex flex-col gap-2">
        <Label htmlFor="mcp-name">Credential name</Label>
        <input id="mcp-name" className={inputClass} value={name} maxLength={200} onChange={(event) => setName(event.target.value)} placeholder="My agent" required />
        <Label htmlFor="mcp-expiration">Expires</Label>
        <input id="mcp-expiration" className={inputClass} type="datetime-local" value={expiration} onChange={(event) => setExpiration(event.target.value)} required />
        <Button type="submit" disabled={busy || loading || !!error} className="self-start">Create credential</Button>
      </form>
      {secret && <div className="flex flex-col gap-2 rounded-md border p-3">
        <p className="text-sm">Copy this credential now. It will not be shown again.</p>
        <Label htmlFor="mcp-secret">New credential</Label>
        <input id="mcp-secret" className={`${inputClass} font-mono`} value={secret.value} readOnly autoComplete="off" />
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => { void navigator.clipboard.writeText(secret.value).then(() => toast.success("Credential copied.")).catch(() => toast.error("Select and copy the credential manually.")); }}>Copy</Button>
          <Button variant="outline" onClick={() => setSecret(undefined)}>Dismiss</Button>
        </div>
      </div>}
      {loading && <p className="text-sm">Loading credentials…</p>}
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      {!loading && !error && credentials.length === 0 && <p className="text-sm text-muted-foreground">No agent credentials yet.</p>}
      {credentials.map((item) => <div key={item.id} className="flex items-center justify-between gap-3 rounded-md border p-3">
        <div className="min-w-0">
          <p className="break-words text-sm font-medium">{item.name}</p>
          <p className="text-xs text-muted-foreground">{new Date(item.expiration).getTime() <= Date.now() ? "Expired" : "Expires"} {new Date(item.expiration).toLocaleString()}</p>
        </div>
        <Button variant="outline" size="sm" disabled={busy} onClick={() => { void revoke(item.id); }}>Revoke</Button>
      </div>)}
    </div>
  );
}
