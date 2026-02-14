import { Database, QueryExecResult } from '@jlongster/sql.js';
export declare type LocalReadState = {
    conversationID: string;
    allReadSeq: number;
};
export declare function localReadState(db: Database): QueryExecResult[];
export declare function getReadStateDB(db: Database, conversationID: string): QueryExecResult[];
export declare function upsertReadStateDB(db: Database, stateJSON: string): QueryExecResult[];
export declare function updateReadStateAllReadSeqDB(db: Database, conversationID: string, allReadSeq: number): QueryExecResult[];
export declare function deleteReadStateDB(db: Database, conversationID: string): QueryExecResult[];
