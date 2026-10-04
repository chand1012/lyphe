import { useState } from "react";
import { Link } from "react-router";
import { MoreHorizontalIcon } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "~/components/ui/alert-dialog";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "~/components/ui/dialog";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSub,
  DropdownMenuSubContent, DropdownMenuSubTrigger, DropdownMenuTrigger,
} from "~/components/ui/dropdown-menu";
import { allTags, folders, update, type Entity } from "~/lib/data";
import { deleteEntity, duplicate } from "~/lib/actions";
import { href } from "~/lib/href";
import { SidebarMenuAction } from "~/components/ui/sidebar";

export function EntityMenu({ entity, showOnHover = false, context = "list" }: { entity: Entity; showOnHover?: boolean; context?: "list" | "page" }) {
  const [renaming, setRenaming] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [title, setTitle] = useState(entity.title);

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          {showOnHover ? (
            <SidebarMenuAction showOnHover aria-label={`${entity.title} actions`}><MoreHorizontalIcon /></SidebarMenuAction>
          ) : (
            <Button variant="ghost" size="icon" className="size-6" aria-label={`${entity.title} actions`}><MoreHorizontalIcon /></Button>
          )}
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {context === "list" && <DropdownMenuItem onSelect={() => { setTitle(entity.title); setRenaming(true); }}>Rename</DropdownMenuItem>}
          {entity.kind === "journal" && (
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>Move to folder</DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                {folders.map((folder) => (
                  <DropdownMenuItem key={folder} onSelect={() => update("journal", entity.id, { folder })}>{folder}</DropdownMenuItem>
                ))}
              </DropdownMenuSubContent>
            </DropdownMenuSub>
          )}
          {context === "list" && <DropdownMenuItem asChild><Link to={href(entity.kind, entity.id)}>Open</Link></DropdownMenuItem>}
          {context === "list" && <DropdownMenuSub>
            <DropdownMenuSubTrigger>Add tag</DropdownMenuSubTrigger>
            <DropdownMenuSubContent>
              {allTags().map((tag) => (
                <DropdownMenuItem key={tag} onSelect={() => update(entity.kind, entity.id, {
                  tags: entity.tags.includes(tag) ? entity.tags.filter((t) => t !== tag) : [...entity.tags, tag],
                })}>{tag}</DropdownMenuItem>
              ))}
            </DropdownMenuSubContent>
          </DropdownMenuSub>}
          <DropdownMenuItem onSelect={() => duplicate(entity)}>Duplicate</DropdownMenuItem>
          <DropdownMenuItem variant="destructive" onSelect={() => setConfirmDelete(true)}>Delete</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={renaming} onOpenChange={setRenaming}>
        <DialogContent>
          <DialogHeader><DialogTitle>Rename {entity.kind}</DialogTitle><DialogDescription>Change the name shown in the sidebar.</DialogDescription></DialogHeader>
          <form onSubmit={(event) => { event.preventDefault(); if (title.trim()) update(entity.kind, entity.id, { title: title.trim() }); setRenaming(false); }} className="flex gap-2">
            <Input autoFocus value={title} onChange={(event) => setTitle(event.target.value)} aria-label="Name" />
            <Button disabled={!title.trim()}>Save</Button>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <AlertDialogContent>
          <AlertDialogHeader><AlertDialogTitle>Delete {entity.title}?</AlertDialogTitle><AlertDialogDescription>You can undo this from the notification.</AlertDialogDescription></AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => deleteEntity(entity)}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
