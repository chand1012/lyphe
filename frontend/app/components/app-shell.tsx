import { useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router";
import { PlusIcon, SearchIcon } from "lucide-react";
import { Button } from "~/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "~/components/ui/dropdown-menu";
import { SidebarInset, SidebarProvider, SidebarTrigger } from "~/components/ui/sidebar";
import { AppSidebar } from "~/components/app-sidebar";
import { CommandSearch } from "~/components/command-search";
import { create, type Kind } from "~/lib/data";
import { href } from "~/lib/href";

const nav: [string, string][] = [["Journal", "/journal"], ["Goals", "/goals"], ["Tasks", "/tasks"], ["Habits", "/habits"]];

function NavLinks() {
  return (
    <nav className="flex items-center gap-1">
      {nav.map(([label, to]) => (
        <NavLink
          key={to}
          to={to}
          className={({ isActive }) =>
            `rounded-md px-2 py-1 text-sm transition-colors ${isActive ? "text-foreground underline decoration-2 underline-offset-4" : "text-muted-foreground hover:text-foreground"}`
          }
        >
          {label}
        </NavLink>
      ))}
    </nav>
  );
}

function CreateMenu() {
  const navigate = useNavigate();

  const createItem = async (kind: Kind) => {
    const entity = await create(kind, { kind, title: `New ${kind}`, tags: [], blocks: [] });
    navigate(href(kind, entity.id));
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Create">
          <PlusIcon />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {(["journal", "task", "goal", "habit"] as Kind[]).map((kind) => (
          <DropdownMenuItem key={kind} onClick={() => createItem(kind)}>
            New {kind[0].toUpperCase() + kind.slice(1)}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function AppShell() {
  const [searchOpen, setSearchOpen] = useState(false);

  return (
    <SidebarProvider>
      <AppSidebar />

      <SidebarInset>
        <header className="flex items-center gap-2 border-b px-4 py-2">
          <SidebarTrigger />
          <div className="hidden md:block">
            <NavLinks />
          </div>
          <div className="ml-auto flex items-center gap-1">
            <Button variant="ghost" size="sm" onClick={() => setSearchOpen(true)}>
              <SearchIcon className="size-4" />
              Search
              <kbd className="hidden text-xs text-muted-foreground sm:inline">⌘K</kbd>
            </Button>
            <CreateMenu />

          </div>
        </header>

        <main className="flex-1 p-4">
          <Outlet />
        </main>
      </SidebarInset>

      <CommandSearch open={searchOpen} onOpenChange={setSearchOpen} />
    </SidebarProvider>
  );
}
