import { Database, QueryExecResult } from '@jlongster/sql.js';
export declare type LocalGroupReadState = {
    conversationID: string;
    minReadSeq: number;
    memberCount: number;
    cursorCount: number;
    lastSyncTime: number;
    version: number;
};
export declare function localGroupReadState(db: Database): QueryExecResult[];
export declare function getGroupReadState(db: Database, conversationID: string): QueryExecResult[];
export declare function upsertGroupReadState(db: Database, stateJSON: string): QueryExecResult[];
export declare function updateGroupReadStateMinSeq(db: Database, conversationID: string, minReadSeq: number): QueryExecResult[];
export declare function deleteGroupReadState(db: Database, conversationID: string): QueryExecResult[];
