import darkUrl from '@/assets/liaison-brand-dark.svg';
import lightUrl from '@/assets/liaison-brand-light.svg';
import './LiaisonBrand.less';

/** Smaller A wordmark for the two-line console header. */
export function LiaisonWordmark() {
  return <svg className="liaison-wordmark" width="66" height="33" viewBox="0 -15 260 130" role="img" aria-label="Liaison">
    <text x="0" y="78" fontFamily="Arial, Helvetica, sans-serif" fontSize="84" fontWeight="600" letterSpacing="-3" fill="currentColor">li<tspan fill="var(--wordmark-ai)" fillOpacity="var(--wordmark-ai-opacity)">ai</tspan>son</text>
    <g transform="translate(-124 0)" fill="none" stroke="var(--wordmark-ai)" strokeOpacity="var(--wordmark-rays-opacity)" strokeWidth="3.6" strokeLinecap="round">
      <path d="M181 6l-4-7 M199 3v-9 M217 6l4-7 M181 94l-4 7 M199 97v9 M217 94l4 7" />
    </g>
  </svg>;
}

/** Theme follows the console preference, not the operating system. */
export function LiaisonBrand({ className = '' }: { className?: string }) {
  return <span className={`liaison-brand-lockup ${className}`} role="img" aria-label="Liaison">
    <img className="liaison-brand-light" src={lightUrl} alt="" aria-hidden="true" width="360" height="117" />
    <img className="liaison-brand-dark" src={darkUrl} alt="" aria-hidden="true" width="360" height="117" />
  </span>;
}
