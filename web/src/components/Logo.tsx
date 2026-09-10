import { useId } from "react"

// An abstracted winged sun disc — the Achaemenid/Zoroastrian-era emblem
// of Ahura Mazda and royal protection carved above the gates at
// Persepolis and Pasargadae, reduced here to a circle and two swept
// wings so it stays legible at favicon size. Pre-Islamic Persian, not a
// generic Middle Eastern motif.
export function Logo({ className }: { className?: string }) {
  const gradientId = useId()

  return (
    <svg viewBox="0 0 64 64" className={className} role="img" aria-label="Vohu">
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="64" y2="64" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#1e3a8a" />
          <stop offset="1" stopColor="#3b82f6" />
        </linearGradient>
      </defs>
      <circle cx="32" cy="32" r="32" fill={`url(#${gradientId})`} />
      <ellipse cx="44" cy="31" rx="11" ry="3.6" fill="#fff" transform="rotate(-6 44 31)" />
      <ellipse cx="53" cy="33.5" rx="7.5" ry="2.8" fill="#fff" transform="rotate(6 53 33.5)" />
      <ellipse cx="20" cy="31" rx="11" ry="3.6" fill="#fff" transform="rotate(6 20 31)" />
      <ellipse cx="11" cy="33.5" rx="7.5" ry="2.8" fill="#fff" transform="rotate(-6 11 33.5)" />
      <circle cx="32" cy="32" r="6.5" fill="#fff" />
    </svg>
  )
}
