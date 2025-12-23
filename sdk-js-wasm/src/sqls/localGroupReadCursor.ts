import { Database, QueryExecResult } from '@jlongster/sql.js';

export type LocalGroupReadCursor = {
  conversationID: string;
  userID: string;
  maxReadSeq: number;
};

export function localGroupReadCursor(db: Database): QueryExecResult[] {
  return db.exec(
    `
      create table if not exists 'local_group_read_cursor' (
            'conversation_id' char(128),
            'user_id' char(64),
            'max_read_seq' integer,
            primary key ('conversation_id', 'user_id')
        )
    `
  );
}

export function insertGroupReadCursor(
  db: Database,
  cursorJSON: string
): QueryExecResult[] {
  const cursor = JSON.parse(cursorJSON) as LocalGroupReadCursor;
  return db.exec(
    `
      insert or replace into local_group_read_cursor (conversation_id, user_id, max_read_seq)
      values ('${cursor.conversationID}', '${cursor.userID}', ${cursor.maxReadSeq});
    `
  );
}

export function getGroupReadCursor(
  db: Database,
  conversationID: string,
  userID: string
): QueryExecResult[] {
  return db.exec(
    `
      select * from local_group_read_cursor
      where conversation_id = '${conversationID}' and user_id = '${userID}'
      limit 1;
    `
  );
}

export function getGroupReadCursorsByConversationID(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  return db.exec(
    `
      select * from local_group_read_cursor
      where conversation_id = '${conversationID}';
    `
  );
}

export function updateGroupReadCursor(
  db: Database,
  conversationID: string,
  userID: string,
  maxReadSeq: number
): QueryExecResult[] {
  return db.exec(
    `
      update local_group_read_cursor
      set max_read_seq = ${maxReadSeq}
      where conversation_id = '${conversationID}' and user_id = '${userID}';
    `
  );
}

export function deleteGroupReadCursor(
  db: Database,
  conversationID: string,
  userID: string
): QueryExecResult[] {
  return db.exec(
    `
      delete from local_group_read_cursor
      where conversation_id = '${conversationID}' and user_id = '${userID}';
    `
  );
}

export function deleteGroupReadCursorsByConversationID(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  return db.exec(
    `
      delete from local_group_read_cursor
      where conversation_id = '${conversationID}';
    `
  );
}

export function upsertGroupReadCursor(
  db: Database,
  cursorJSON: string
): QueryExecResult[] {
  const cursor = JSON.parse(cursorJSON) as LocalGroupReadCursor;
  return db.exec(
    `
      insert or replace into local_group_read_cursor (conversation_id, user_id, max_read_seq)
      values ('${cursor.conversationID}', '${cursor.userID}', ${cursor.maxReadSeq});
    `
  );
}

export function getMinReadSeqFromCursors(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  return db.exec(
    `
      select min(max_read_seq) as min_read_seq from local_group_read_cursor
      where conversation_id = '${conversationID}';
    `
  );
}
