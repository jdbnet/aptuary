export const ALL_SCOPES = [
  'packages:read', 'packages:write',
  'repos:read', 'repos:write',
  'users:read', 'users:write',
  'apikeys:read', 'apikeys:write',
  'audit:read',
  'stats:read',
  'settings:read', 'settings:write',
  'gpg:read', 'gpg:write',
]

export const SCOPE_LABELS = {
  'packages:read': 'Packages (read)',
  'packages:write': 'Packages (write)',
  'repos:read': 'Repositories (read)',
  'repos:write': 'Repositories (write)',
  'users:read': 'Users (read)',
  'users:write': 'Users (write)',
  'apikeys:read': 'API keys (read)',
  'apikeys:write': 'API keys (write)',
  'audit:read': 'Audit (read)',
  'stats:read': 'Stats (read)',
  'settings:read': 'Settings (read)',
  'settings:write': 'Settings (write)',
  'gpg:read': 'GPG (read)',
  'gpg:write': 'GPG (write)',
}
