import { type RouteConfig, index, layout, route } from "@react-router/dev/routes";

export default [
  index("routes/home.tsx"),
  route("login", "routes/login.tsx"),
  route("register", "routes/register.tsx"),
  layout("routes/app.tsx", [
    route("journal/:id?", "routes/journal.tsx"),
    route("goals", "routes/goals.tsx"),
    route("goals/:id", "routes/goal.tsx"),
    route("tasks", "routes/tasks.tsx"),
    route("habits", "routes/habits.tsx"),
    route("habits/:id", "routes/habit.tsx"),
    route("settings", "routes/settings.tsx"),
  ]),
];
