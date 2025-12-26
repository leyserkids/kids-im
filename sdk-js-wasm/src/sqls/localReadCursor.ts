import { Database, QueryExecResult } from '@jlongster/sql.js';

export type LocalReadCursor = {
  conversationID: string;
  userID: string;
  maxReadSeq: number;
};

export function localReadCursor(db: Database): QueryExecResult[] {
  return db.exec(
    `
      create table if not exists 'local_read_cursor' (
            'conversation_id' char(128),
            'user_id' char(64),
            'max_read_seq' integer,
            primary key ('conversation_id', 'user_id')
        )
    `
  );
}

export function insertReadCursor(
  db: Database,
  cursorJSON: string
): QueryExecResult[] {
  const cursor = JSON.parse(cursorJSON) as LocalReadCursor;
  return db.exec(
    `
      insert or replace into local_read_cursor (conversation_id, user_id, max_read_seq)
      values ('${cursor.conversationID}', '${cursor.userID}', ${cursor.maxReadSeq});
    `
  );
}

export function getReadCursor(
  db: Database,
  conversationID: string,
  userID: string
): QueryExecResult[] {
  return db.exec(
    `
      select * from local_read_cursor
      where conversation_id = '${conversationID}' and user_id = '${userID}'
      limit 1;
    `
  );
}

export function getReadCursorsByConversationID(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  return db.exec(
    `
      select * from local_read_cursor
      where conversation_id = '${conversationID}';
    `
  );
}

export function updateReadCursor(
  db: Database,
  conversationID: string,
  userID: string,
  maxReadSeq: number
): QueryExecResult[] {
  return db.exec(
    `
      update local_read_cursor
      set max_read_seq = ${maxReadSeq}
      where conversation_id = '${conversationID}' and user_id = '${userID}';
    `
  );
}

export function deleteReadCursor(
  db: Database,
  conversationID: string,
  userID: string
): QueryExecResult[] {
  return db.exec(
    `
      delete from local_read_cursor
      where conversation_id = '${conversationID}' and user_id = '${userID}';
    `
  );
}

export function deleteReadCursorsByConversationID(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  return db.exec(
    `
      delete from local_read_cursor
      where conversation_id = '${conversationID}';
    `
  );
}

export function upsertReadCursor(
  db: Database,
  cursorJSON: string
): QueryExecResult[] {
  const cursor = JSON.parse(cursorJSON) as LocalReadCursor;
  return db.exec(
    `
      insert or replace into local_read_cursor (conversation_id, user_id, max_read_seq)
      values ('${cursor.conversationID}', '${cursor.userID}', ${cursor.maxReadSeq});
    `
  );
}

export function getAllReadSeqFromCursors(
  db: Database,
  conversationID: string,
  excludeUserID: string
): QueryExecResult[] {
  return db.exec(
    `
      select min(max_read_seq) as all_read_seq from local_read_cursor
      where conversation_id = '${conversationID}' and user_id != '${excludeUserID}';
    `
  );
}
