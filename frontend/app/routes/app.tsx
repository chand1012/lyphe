import { redirect, useBlocker } from "react-router";
import { AppShell } from "~/components/app-shell";
import { useEffect } from "react";
import { connectData, flushAll, hasPendingChanges, loadData, useDb } from "~/lib/data";
import { pb } from "~/lib/pocketbase";

export async function clientLoader() {
  if (!pb.authStore.isValid) throw redirect("/login");
  await loadData();
}

export default function App() {
  useDb();
  const blocker = useBlocker(() => hasPendingChanges());
  useEffect(() => { if (blocker.state === "blocked") { void flushAll().then(() => blocker.proceed()).catch(() => blocker.reset()); } }, [blocker]);
  useEffect(() => { void connectData().catch(() => {}); return () => { void flushAll().catch(() => {}); }; }, []);
  return <AppShell />;
}
