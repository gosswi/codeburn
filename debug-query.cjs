const Database = require('better-sqlite3')
const path = require('path')
const os = require('os')

const dbPath = path.join(os.homedir(), 'Library', 'Application Support', 'Cursor', 'User', 'globalStorage', 'state.vscdb')
const db = new Database(dbPath, { readonly: true })

const floor = new Date(Date.now() - 120 * 24 * 60 * 60 * 1000).toISOString()
console.log('Time floor:', floor)

const rows = db.prepare(`
  SELECT
    json_extract(value, '$.tokenCount.inputTokens') as it,
    json_extract(value, '$.tokenCount.outputTokens') as ot,
    json_extract(value, '$.createdAt') as ts
  FROM cursorDiskKV
  WHERE key LIKE 'bubbleId:%'
    AND json_extract(value, '$.tokenCount.inputTokens') > 0
    AND json_extract(value, '$.createdAt') > ?
  ORDER BY json_extract(value, '$.createdAt') DESC
  LIMIT 5
`).all(floor)

console.log('Rows found:', rows.length)
console.log('Sample:', rows)
db.close()
