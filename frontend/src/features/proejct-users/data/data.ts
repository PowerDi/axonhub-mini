import { IconCash, IconShield, IconUsersGroup, IconUserShield } from '@tabler/icons-react';

// export const callTypes = new Map<UserStatus, string>([
//   ['active', 'bg-teal-100/30 text-teal-900 dark:text-teal-200 border-teal-200'],
//   ['inactive', 'bg-muted border-border'],
//   ['invited', 'bg-info/40 text-(--info-soft-fg) dark:text-(--info-soft-fg) border-info/40'],
//   [
//     'suspended',
//     'bg-destructive/10 dark:bg-destructive/50 text-destructive dark:text-primary border-destructive/10',
//   ],
// ])

export const userTypes = [
  {
    label: 'Superadmin',
    value: 'superadmin',
    icon: IconShield,
  },
  {
    label: 'Admin',
    value: 'admin',
    icon: IconUserShield,
  },
  {
    label: 'Manager',
    value: 'manager',
    icon: IconUsersGroup,
  },
  {
    label: 'Cashier',
    value: 'cashier',
    icon: IconCash,
  },
] as const;
