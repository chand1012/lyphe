import type { Route } from "./+types/home";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "Lyphe" },
    { name: "description", content: "Life dashboard" },
  ];
}

export default function Home() {
  return (
    <main className="mx-auto grid min-h-screen place-items-center p-4 container">
      <Card className="max-w-md">
        <CardHeader>
          <CardTitle>Lyphe</CardTitle>
          <CardDescription>Example shadcn page.</CardDescription>
        </CardHeader>
        <CardContent className="flex gap-2">
          <Button>Goals</Button>
          <Button variant="outline">Journal</Button>
        </CardContent>
      </Card>
    </main>
  );
}
