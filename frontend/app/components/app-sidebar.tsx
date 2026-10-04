import { toast } from "sonner";
import { useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import { ChevronRightIcon, ChevronUpIcon, FolderPlusIcon, LogOutIcon, PlusIcon, SettingsIcon } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "~/components/ui/collapsible";
import { Popover, PopoverContent, PopoverTrigger } from "~/components/ui/popover";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "~/components/ui/dropdown-menu";
import { pb } from "~/lib/pocketbase";
import { Avatar, AvatarFallback } from "~/components/ui/avatar";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  SidebarRail,
  useSidebar,
} from "~/components/ui/sidebar";
import { EntityMenu } from "~/components/entity-menu";
import { useAuth } from "~/lib/auth";
import { create, createFolder, flushAll, folders, update, useDb, type Entity } from "~/lib/data";
import { dayLabel } from "~/lib/format";
import { href } from "~/lib/href";

function Row({ entity, label, sub = false }: { entity: Entity; label: string; sub?: boolean }) {
  const path = href(entity.kind, entity.id);
  const active = useLocation().pathname === path;
  const { setOpenMobile } = useSidebar();
  const Item = sub ? SidebarMenuSubItem : SidebarMenuItem;

  return (
    <Item>
      {sub ? (
        <SidebarMenuSubButton asChild size="sm" isActive={active}>
          <Link to={path} onClick={() => setOpenMobile(false)}>{label}</Link>
        </SidebarMenuSubButton>
      ) : (
        <SidebarMenuButton asChild size="sm" isActive={active}>
          <Link to={path} onClick={() => setOpenMobile(false)}>{label}</Link>
        </SidebarMenuButton>
      )}
      <EntityMenu entity={entity} showOnHover />
    </Item>
  );
}

function JournalTree() {
  const db = useDb();
  const navigate = useNavigate();
  const { setOpenMobile } = useSidebar();
  const [folderOpen, setFolderOpen] = useState(false);
  const [folderName, setFolderName] = useState("");
  const entries = [...db.journal].sort((a, b) => b.date.localeCompare(a.date));
  const folderNames = [...new Set([...folders, ...entries.map((e) => e.folder)])];
  const newEntry = async (folder = "Unfiled") => {
    const entry = await create("journal", { kind: "journal", title: "Untitled", tags: [], blocks: [] });
    update("journal", entry.id, { folder });
    navigate(`/journal/${entry.id}`);
    setOpenMobile(false);
  };

  return (
    <SidebarGroup>
      <div className="flex items-center justify-between pr-2">
        <SidebarGroupLabel>Journal</SidebarGroupLabel>
        <div className="flex items-center">
          <Button variant="ghost" size="icon" className="size-6" aria-label="New journal" onClick={() => newEntry()}><PlusIcon /></Button>
          <Popover open={folderOpen} onOpenChange={setFolderOpen}>
            <PopoverTrigger asChild><Button variant="ghost" size="icon" className="size-6" aria-label="New folder"><FolderPlusIcon /></Button></PopoverTrigger>
            <PopoverContent align="start" className="w-52">
              <form onSubmit={(event) => { event.preventDefault(); createFolder(folderName); setFolderName(""); setFolderOpen(false); }} className="flex gap-2">
                <Input autoFocus value={folderName} onChange={(event) => setFolderName(event.target.value)} placeholder="Folder name" aria-label="Folder name" />
                <Button size="sm" disabled={!folderName.trim()}>Add</Button>
              </form>
            </PopoverContent>
          </Popover>
        </div>
      </div>
      <SidebarGroupContent>
        <SidebarMenu>
          {folderNames.map((folder) => (
            <Collapsible key={folder} asChild defaultOpen className="group/folder">
              <SidebarMenuItem>
                <CollapsibleTrigger asChild>
                  <SidebarMenuButton size="sm"><ChevronRightIcon className="size-3 transition-transform group-data-[state=open]/folder:rotate-90" />{folder}</SidebarMenuButton>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <SidebarMenuSub>
                    {entries.filter((e) => e.folder === folder).map((entry) => (
                      <Row key={entry.id} entity={entry} label={dayLabel(entry.date)} sub />
                    ))}
                    <SidebarMenuSubItem>
                      <SidebarMenuSubButton asChild size="sm"><button onClick={() => newEntry(folder)}>+ New</button></SidebarMenuSubButton>
                    </SidebarMenuSubItem>
                  </SidebarMenuSub>
                </CollapsibleContent>
              </SidebarMenuItem>
            </Collapsible>
          ))}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  );
}

function Section({ title, items }: { title: string; items: Entity[] }) {
  return (
    <SidebarGroup>
      <SidebarGroupLabel>{title}</SidebarGroupLabel>
      <SidebarGroupContent>
        <SidebarMenu>
          {items.map((entity) => (
            <Row key={entity.id} entity={entity} label={entity.title} />
          ))}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  );
}

export function AppSidebar() {
  const db = useDb();
  const user = useAuth();
  const navigate = useNavigate();
  const [loggingOut, setLoggingOut] = useState(false);
  const logOut = async () => {
    setLoggingOut(true);
    try {
      await flushAll();
      pb.authStore.clear();
      setOpenMobile(false);
      navigate("/login");
    } catch {
      toast.error("Save your pending changes before logging out");
    } finally { setLoggingOut(false); }
  };
  const location = useLocation();
  const { setOpenMobile } = useSidebar();

  return (
    <Sidebar collapsible="offcanvas">
      <SidebarHeader>
        <div className="flex items-center justify-between gap-2 px-1">
          <span className="text-sm font-medium">Lyphe</span>
        </div>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup className="md:hidden">
          <SidebarGroupLabel>Navigate</SidebarGroupLabel>
          <SidebarMenu>
            {[["Journal", "/journal"], ["Goals", "/goals"], ["Tasks", "/tasks"], ["Habits", "/habits"]].map(([label, to]) => (
              <SidebarMenuItem key={to}>
                <SidebarMenuButton asChild isActive={location.pathname === to}>
                  <Link to={to} onClick={() => setOpenMobile(false)}>{label}</Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
            ))}
          </SidebarMenu>
        </SidebarGroup>
        <JournalTree />
        <Section title="Goals" items={db.goal} />
        <Section title="Tasks" items={db.task} />
        <Section title="Habits" items={db.habit} />
      </SidebarContent>

      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton className="h-11 gap-2 data-[state=open]:bg-sidebar-accent" disabled={loggingOut} aria-label={`Account menu for ${user?.name ?? "your account"}`}>
                  <Avatar className="size-7">
                    <AvatarFallback>{(user?.name ?? "?").slice(0, 1).toUpperCase()}</AvatarFallback>
                  </Avatar>
                  <span className="truncate text-xs">{user?.name ?? "Account"}</span>
                  <ChevronUpIcon className="ml-auto size-4 text-muted-foreground" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="top" align="start" sideOffset={8} className="w-[var(--radix-dropdown-menu-trigger-width)] min-w-48">
                <DropdownMenuItem asChild>
                  <Link to="/settings" onClick={() => setOpenMobile(false)}><SettingsIcon className="size-4" />Settings</Link>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={() => { void logOut(); }} disabled={loggingOut}>
                  <LogOutIcon className="size-4" />Log out
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>

      <SidebarRail />
    </Sidebar>
  );
}
