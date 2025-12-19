import { Database, QueryExecResult } from '@jlongster/sql.js';

export type LocalGroupReadCursorState = {
  conversationID: string;
  cursorVersion: number;
};

export function localGroupReadCursorState(db: Database): QueryExecResult[] {
  return db.exec(
    `
      create table if not exists 'local_group_read_cursor_state' (
            'conversation_id' char(128),
            'cursor_version' integer default 0,
            primary key ('conversation_id')
        )
    `
  );
}

export function insertGroupReadCursorState(
  db: Database,
  stateJSON: string
): QueryExecResult[] {
  const state = JSON.parse(stateJSON) as LocalGroupReadCursorState;
  return db.exec(
    `
      insert or replace into local_group_read_cursor_state (conversation_id, cursor_version)
      values ('${state.conversationID}', ${state.cursorVersion || 0});
    `
  );
}

export function getGroupReadCursorState(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  return db.exec(
    `
      select * from local_group_read_cursor_state
      where conversation_id = '${conversationID}'
      limit 1;
    `
  );
}

export function deleteGroupReadCursorState(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  return db.exec(
    `
      delete from local_group_read_cursor_state
      where conversation_id = '${conversationID}';
    `
  );
}

export function incrementGroupReadCursorVersion(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  // First ensure the record exists
  db.exec(
    `
      insert or ignore into local_group_read_cursor_state (conversation_id, cursor_version)
      values ('${conversationID}', 0);
    `
  );

  // Then increment
  return db.exec(
    `
      update local_group_read_cursor_state
      set cursor_version = cursor_version + 1
      where conversation_id = '${conversationID}';
    `
  );
}
