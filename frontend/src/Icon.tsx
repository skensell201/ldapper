import type { ReactNode } from "react";

/** The names here are exactly the ones app/icon.go can return. A rule added
 *  there without a path here draws nothing at all. */
const paths: Record<string, ReactNode> = {
  domain: (
    <>
      <circle cx="8" cy="8" r="5.8" />
      <path d="M2.2 8h11.6M8 2.2c2.4 2.7 2.4 9.1 0 11.6M8 2.2C5.6 4.9 5.6 11.3 8 13.8" />
    </>
  ),
  ou: <path d="M2.2 12.8V4.2h3.9l1.3 1.7h6.4v6.9z" />,
  container: (
    <>
      <rect x="2.4" y="3.6" width="11.2" height="8.8" rx="1.6" />
      <path d="M2.4 6.6h11.2" />
    </>
  ),
  user: (
    <>
      <circle cx="8" cy="5.4" r="2.6" />
      <path d="M2.8 13.6c0-2.9 2.3-4.4 5.2-4.4s5.2 1.5 5.2 4.4" />
    </>
  ),
  group: (
    <>
      <circle cx="6.1" cy="5.6" r="2.3" />
      <path d="M1.6 13.4c0-2.6 2-4 4.5-4s4.5 1.4 4.5 4" />
      <path d="M10.6 3.6a2.3 2.3 0 0 1 0 4.4M11.6 9.7c1.7.4 2.8 1.6 2.8 3.7" />
    </>
  ),
  computer: (
    <>
      <rect x="2.2" y="3.4" width="11.6" height="7.4" rx="1.4" />
      <path d="M5.6 13.2h4.8" />
    </>
  ),
  object: <circle cx="8" cy="8" r="4.4" />,
};

export function Icon({ name, className }: { name: string; className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.2"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[name] ?? paths.object}
    </svg>
  );
}
