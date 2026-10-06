import type { ThemePref } from "./theme";

const OPTIONS: { value: ThemePref; label: string; icon: React.ReactNode }[] = [
  {
    value: "system",
    label: "Follow the system",
    icon: (
      <>
        <rect x="2.2" y="3" width="11.6" height="7.8" rx="1.4" />
        <path d="M5.6 13.2h4.8M8 10.8v2.4" />
      </>
    ),
  },
  {
    value: "light",
    label: "Light",
    icon: (
      <>
        <circle cx="8" cy="8" r="2.8" />
        <path d="M8 1.6v1.6M8 12.8v1.6M1.6 8h1.6M12.8 8h1.6M3.5 3.5l1.1 1.1M11.4 11.4l1.1 1.1M3.5 12.5l1.1-1.1M11.4 4.6l1.1-1.1" />
      </>
    ),
  },
  {
    value: "dark",
    label: "Dark",
    icon: <path d="M13.2 9.6A5.6 5.6 0 0 1 6.4 2.8a5.6 5.6 0 1 0 6.8 6.8z" />,
  },
];

/** Three icons in a pill: follow the system, or pin light or dark. */
export function ThemeSwitch({
  value,
  onChange,
}: {
  value: ThemePref;
  onChange: (v: ThemePref) => void;
}) {
  return (
    <div className="segs theme-switch no-drag" role="radiogroup" aria-label="Theme">
      {OPTIONS.map((o) => (
        <button
          key={o.value}
          className={value === o.value ? "seg on" : "seg"}
          onClick={() => onChange(o.value)}
          role="radio"
          aria-checked={value === o.value}
          aria-label={o.label}
          title={o.label}
        >
          <svg
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.3"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            {o.icon}
          </svg>
        </button>
      ))}
    </div>
  );
}
