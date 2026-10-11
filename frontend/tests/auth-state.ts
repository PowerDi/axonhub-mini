import { fileURLToPath } from 'node:url'

export const adminAuthFile = fileURLToPath(new URL('../playwright/.auth/admin.json', import.meta.url))
