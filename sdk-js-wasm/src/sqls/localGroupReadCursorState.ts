import { Database, QueryExecResult } from '@jlongster/sql.js';

export type LocalGroupReadState = {
  conversationID: string;
  minReadSeq: number;
  memberCount: number;
  cursorCount: number;
  lastSyncTime: number;
  version: number;
};

export function localGroupReadState(db: Database): QueryExecResult[] {
  return db.exec(
    `
      create table if not exists 'local_group_read_state' (
            'conversation_id' char(128),
            'min_read_seq' integer default 0,
            'member_count' integer default 0,
            'cursor_count' integer default 0,
            'last_sync_time' integer default 0,
            'version' integer default 0,
            primary key ('conversation_id')
        )
    `
  );
}

export function getGroupReadState(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  return db.exec(
    `
      select * from local_group_read_state
      where conversation_id = '${conversationID}'
      limit 1;
    `
  );
}

export function upsertGroupReadState(
  db: Database,
  stateJSON: string
): QueryExecResult[] {
  const state = JSON.parse(stateJSON) as LocalGroupReadState;
  return db.exec(
    `
      insert or replace into local_group_read_state (conversation_id, min_read_seq, member_count, cursor_count, last_sync_time, version)
      values ('${state.conversationID}', ${state.minReadSeq || 0}, ${
      state.memberCount || 0
    }, ${state.cursorCount || 0}, ${state.lastSyncTime || 0}, ${
      state.version || 0
    });
    `
  );
}

export function updateGroupReadStateMinSeq(
  db: Database,
  conversationID: string,
  minReadSeq: number
): QueryExecResult[] {
  return db.exec(
    `
      update local_group_read_state
      set min_read_seq = ${minReadSeq}
      where conversation_id = '${conversationID}';
    `
  );
}

export function deleteGroupReadState(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  return db.exec(
    `
      delete from local_group_read_state
      where conversation_id = '${conversationID}';
    `
  );
}
