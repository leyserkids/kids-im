import { Database, QueryExecResult } from '@jlongster/sql.js';
export declare type LocalReadCursor = {
    conversationID: string;
    userID: string;
    maxReadSeq: number;
};
export declare function localReadCursor(db: Database): QueryExecResult[];
export declare function insertReadCursor(db: Database, cursorJSON: string): QueryExecResult[];
export declare function getReadCursor(db: Database, conversationID: string, userID: string): QueryExecResult[];
export declare function getReadCursorsByConversationID(db: Database, conversationID: string): QueryExecResult[];
export declare function updateReadCursor(db: Database, conversationID: string, userID: string, maxReadSeq: number): QueryExecResult[];
export declare function deleteReadCursor(db: Database, conversationID: string, userID: string): QueryExecResult[];
export declare function deleteReadCursorsByConversationID(db: Database, conversationID: string): QueryExecResult[];
export declare function upsertReadCursor(db: Database, cursorJSON: string): QueryExecResult[];
export declare function getAllReadSeqFromCursors(db: Database, conversationID: string, excludeUserID: string): QueryExecResult[];
