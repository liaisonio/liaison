import type { LucideProps } from 'lucide-react';
import { FileSearch } from 'lucide-react';

export function AuditLogIcon({ size = 17, strokeWidth = 1.8, ...props }: LucideProps) {
  return <FileSearch size={size} strokeWidth={strokeWidth} {...props} />;
}
