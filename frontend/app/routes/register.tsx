import { data, Form, redirect } from "react-router";
import { ClientResponseError } from "pocketbase";
import type { Route } from "./+types/register";
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
  const email = String(form.get("email") ?? "");
  const password = String(form.get("password") ?? "");
  const passwordConfirm = String(form.get("passwordConfirm") ?? "");

  if (password !== passwordConfirm) {
    return data({ error: "Passwords do not match." }, { status: 400 });
  }

  try {
    await pb.collection("users").create({
      email,
      password,
      passwordConfirm,
      name: String(form.get("name") ?? ""),
    });
    await pb.collection("users").authWithPassword(email, password);
  } catch (e) {
    return data(
      { error: e instanceof ClientResponseError ? e.message : "Registration failed." },
      { status: 400 },
    );
  }

  return redirect("/");
}

export default function Register({ actionData }: Route.ComponentProps) {
  return (
    <main className="mx-auto grid min-h-screen place-items-center p-4 container">
      <Card className="max-w-md">
        <CardHeader>
          <CardTitle>Create an account</CardTitle>
          <CardDescription>
            Already have one? <a href="/login" className="text-primary">Sign in</a>
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form method="post" className="flex flex-col gap-3">
            <Label htmlFor="name">Name</Label>
            <Input id="name" name="name" required />

            <Label htmlFor="email">Email</Label>
            <Input id="email" name="email" type="email" required autoComplete="email" />

            <Label htmlFor="password">Password</Label>
            <Input id="password" name="password" type="password" required minLength={8} />

            <Label htmlFor="passwordConfirm">Confirm password</Label>
            <Input id="passwordConfirm" name="passwordConfirm" type="password" required minLength={8} />

            {actionData?.error && <p className="text-destructive">{actionData.error}</p>}

            <Button type="submit">Register</Button>
          </Form>
        </CardContent>
      </Card>
    </main>
  );
}
