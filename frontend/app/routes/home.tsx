import { redirect } from "react-router";
import { pb } from "~/lib/pocketbase";

export async function clientLoader() {
  throw redirect(pb.authStore.isValid ? "/journal" : "/login");
}

export default function Home() {
  return null;
}
