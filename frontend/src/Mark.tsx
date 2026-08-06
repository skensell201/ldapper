/** The Ldapper mark: two levels of a directory tree, which is also the L the
 *  name starts with. Kept in sync with assets/logo.svg by hand — it is five
 *  shapes and has not changed since it was drawn. */
export function Mark({ size = 18 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true" className="mark">
      <g fill="none" stroke="#5c5f85" strokeWidth="4" strokeLinecap="round">
        <path d="M15 20 V27 Q15 32 20 32 H26" />
        <path d="M32 37 V44 Q32 49 37 49 H43" />
      </g>
      <circle cx="15" cy="15" r="4" fill="#8e8da0" />
      <circle cx="32" cy="32" r="4" fill="#a3a3a7" />
      <circle cx="49" cy="49" r="5.5" fill="#ff4a36" />
    </svg>
  );
}
