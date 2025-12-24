import { Database, QueryExecResult } from '@jlongster/sql.js';

export type LocalGroupReadState = {
  conversationID: string;
  minReadSeq: number;
};

export function localGroupReadState(db: Database): QueryExecResult[] {
  return db.exec(
    `
      create table if not exists 'local_group_read_state' (
            'conversation_id' char(128),
            'min_read_seq' integer default 0,
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
      insert or replace into local_group_read_state (conversation_id, min_read_seq)
      values ('${state.conversationID}', ${state.minReadSeq || 0});
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
