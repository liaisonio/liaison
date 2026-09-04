import liaisonMarkUrl from '../../../desktop-client/public/liaison-mark.svg';

type LiaisonLogoProps = {
  size?: number;
  className?: string;
  title?: string;
  decorative?: boolean;
};

/**
 * Uses the repository's canonical desktop-client logo asset directly.
 */
export function LiaisonLogo({
  size = 32,
  className,
  title = 'Liaison',
  decorative = false,
}: LiaisonLogoProps) {
  return (
    <img
      src={liaisonMarkUrl}
      width={size}
      height={size}
      className={className}
      alt={decorative ? '' : title}
      aria-hidden={decorative || undefined}
    />
  );
}
