import { Database, QueryExecResult } from '@jlongster/sql.js';

export type LocalReadState = {
  conversationID: string;
  allReadSeq: number;
};

export function localReadState(db: Database): QueryExecResult[] {
  return db.exec(
    `
      create table if not exists 'local_read_state' (
            'conversation_id' char(128),
            'all_read_seq' integer default 0,
            primary key ('conversation_id')
        )
    `
  );
}

export function getReadStateDB(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  return db.exec(
    `
      select * from local_read_state
      where conversation_id = '${conversationID}'
      limit 1;
    `
  );
}

export function upsertReadStateDB(
  db: Database,
  stateJSON: string
): QueryExecResult[] {
  const state = JSON.parse(stateJSON) as LocalReadState;
  return db.exec(
    `
      insert or replace into local_read_state (conversation_id, all_read_seq)
      values ('${state.conversationID}', ${state.allReadSeq || 0});
    `
  );
}

export function updateReadStateAllReadSeqDB(
  db: Database,
  conversationID: string,
  allReadSeq: number
): QueryExecResult[] {
  return db.exec(
    `
      update local_read_state
      set all_read_seq = ${allReadSeq}
      where conversation_id = '${conversationID}';
    `
  );
}

export function deleteReadStateDB(
  db: Database,
  conversationID: string
): QueryExecResult[] {
  return db.exec(
    `
      delete from local_read_state
      where conversation_id = '${conversationID}';
    `
  );
}
