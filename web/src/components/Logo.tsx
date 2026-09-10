import { useId } from "react"

// A spark above a converging "V" — thought (the spark) resolving into
// action (the V), and the initial letter of the name. Single-color mark
// on a warm gradient badge so it stays legible at favicon size.
export function Logo({ className }: { className?: string }) {
  const gradientId = useId()

  return (
    <svg viewBox="0 0 64 64" className={className} role="img" aria-label="Vohu">
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="64" y2="64" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#fbbf24" />
          <stop offset="1" stopColor="#ea580c" />
        </linearGradient>
      </defs>
      <circle cx="32" cy="32" r="32" fill={`url(#${gradientId})`} />
      <circle cx="32" cy="13.5" r="3.4" fill="#fff" />
      <path
        d="M20 21 L32 43 L44 21"
        fill="none"
        stroke="#fff"
        strokeWidth="7"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}
