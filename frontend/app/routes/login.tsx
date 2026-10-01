import { data, Form, redirect } from "react-router";
import { ClientResponseError } from "pocketbase";
import type { Route } from "./+types/login";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { pb } from "~/lib/pocketbase";

export async function clientAction({ request }: Route.ClientActionArgs) {
  if (pb.authStore.isValid) return redirect("/");

  const form = await request.formData();

  try {
    await pb.collection("users").authWithPassword(
      String(form.get("email") ?? ""),
      String(form.get("password") ?? ""),
    );
  } catch (e) {
    return data(
      { error: e instanceof ClientResponseError ? e.message : "Login failed." },
      { status: 400 },
    );
  }

  return redirect("/");
}

export default function Login({ actionData }: Route.ComponentProps) {
  return (
    <main className="mx-auto grid min-h-screen place-items-center p-4 container">
      <Card className="max-w-md">
        <CardHeader>
          <CardTitle>Sign in</CardTitle>
          <CardDescription>
            No account yet? <a href="/register" className="text-primary">Register</a>
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form method="post" className="flex flex-col gap-3">
            <Label htmlFor="email">Email</Label>
            <Input id="email" name="email" type="email" required autoComplete="email" />

            <Label htmlFor="password">Password</Label>
            <Input id="password" name="password" type="password" required minLength={8} />

            {actionData?.error && <p className="text-destructive">{actionData.error}</p>}

            <Button type="submit">Sign in</Button>
          </Form>
        </CardContent>
      </Card>
    </main>
  );
}
