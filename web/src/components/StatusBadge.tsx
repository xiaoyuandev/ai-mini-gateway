interface StatusBadgeProps {
  tone?: "neutral" | "success" | "warning" | "danger"
  children: string
}

export function StatusBadge({ tone = "neutral", children }: StatusBadgeProps) {
  const toneClassName =
    tone === "success"
      ? "badge-success"
      : tone === "warning"
        ? "badge-warning"
        : tone === "danger"
          ? "badge-danger"
          : "badge-neutral"

  return <span className={`inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold ${toneClassName}`}>{children}</span>
}
