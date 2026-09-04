import { Package, type LucideProps } from 'lucide-react';

/** Application resource icon shared by navigation and resource tables. */
export const ApplicationIcon = ({ size = 17, ...props }: LucideProps) => (
  <Package size={size} strokeWidth={1.8} {...props} />
);
