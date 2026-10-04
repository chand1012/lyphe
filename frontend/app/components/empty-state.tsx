import { Link } from "react-router";
import { Button } from "~/components/ui/button";

export function EmptyState({
  title,
  hint,
  action,
}: {
  title: string;
  hint?: string;
  action?: { label: string; href?: string; onClick?: () => void };
}) {
  return (
    <div className="flex flex-col items-start gap-2 py-8 text-muted-foreground">
      <p className="text-sm font-medium">{title}</p>
      {hint && <p className="text-sm">{hint}</p>}
      {action && (action.onClick ? (
        <Button variant="outline" size="sm" className="mt-1" onClick={action.onClick}>{action.label}</Button>
      ) : action.href ? (
        <Button variant="outline" size="sm" asChild className="mt-1"><Link to={action.href}>{action.label}</Link></Button>
      ) : null)}
    </div>
  );
}
