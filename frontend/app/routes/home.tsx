import { redirect, useNavigate } from "react-router";
import type { Route } from "./+types/home";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { pb } from "~/lib/pocketbase";
import { useAuth } from "~/lib/auth";

export async function clientLoader({}: Route.ClientLoaderArgs) {
  if (!pb.authStore.isValid) throw redirect("/login");
}

export function meta({}: Route.MetaArgs) {
  return [
    { title: "Lyphe" },
    { name: "description", content: "Life dashboard" },
  ];
}

export default function Home() {
  const user = useAuth();
  const navigate = useNavigate();

  return (
    <main className="mx-auto grid min-h-screen place-items-center p-4 container">
      <Card className="max-w-md">
        <CardHeader>
          <CardTitle>Lyphe</CardTitle>
          <CardDescription>
            Signed in as <span className="font-content">{user?.name || user?.email}</span>
          </CardDescription>
        </CardHeader>
        <CardContent className="flex gap-2">
          <Button>Goals</Button>
          <Button variant="outline">Journal</Button>
          <Button
            variant="ghost"
            onClick={() => {
              pb.authStore.clear();
              navigate("/login");
            }}
          >
            Sign out
          </Button>
        </CardContent>
      </Card>
    </main>
  );
}
