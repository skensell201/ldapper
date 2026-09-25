import { useId } from "react";

/** The Ldapper mark: two levels of a directory tree, which is also the L the
 *  name starts with, inked in one gradient from the root to the leaf. Kept in
 *  sync with assets/logo.svg by hand — it is five shapes and one gradient.
 *
 *  The gradient needs an id, and the mark can be on screen more than once, so
 *  each instance makes its own. */
export function Mark({ size = 18 }: { size?: number }) {
  const ink = `ink-${useId()}`;
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true" className="mark">
      <defs>
        <linearGradient id={ink} x1="12" y1="12" x2="54" y2="54" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#855aff" />
          <stop offset="0.55" stopColor="#ff7ad9" />
          <stop offset="1" stopColor="#ff5632" />
        </linearGradient>
      </defs>
      <g fill="none" stroke={`url(#${ink})`} strokeWidth="4" strokeLinecap="round">
        <path d="M15 20 V27 Q15 32 20 32 H26" />
        <path d="M32 37 V44 Q32 49 37 49 H43" />
      </g>
      <g fill={`url(#${ink})`}>
        <circle cx="15" cy="15" r="4" />
        <circle cx="32" cy="32" r="4" />
        <circle cx="49" cy="49" r="5.5" />
      </g>
    </svg>
  );
}
