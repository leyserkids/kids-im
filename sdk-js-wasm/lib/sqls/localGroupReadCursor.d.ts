import { Database, QueryExecResult } from '@jlongster/sql.js';
export declare type LocalGroupReadCursor = {
    conversationID: string;
    userID: string;
    maxReadSeq: number;
};
export declare function localGroupReadCursor(db: Database): QueryExecResult[];
export declare function insertGroupReadCursor(db: Database, cursorJSON: string): QueryExecResult[];
export declare function getGroupReadCursor(db: Database, conversationID: string, userID: string): QueryExecResult[];
export declare function getGroupReadCursorsByConversationID(db: Database, conversationID: string): QueryExecResult[];
export declare function updateGroupReadCursor(db: Database, conversationID: string, userID: string, maxReadSeq: number): QueryExecResult[];
export declare function deleteGroupReadCursor(db: Database, conversationID: string, userID: string): QueryExecResult[];
export declare function deleteGroupReadCursorsByConversationID(db: Database, conversationID: string): QueryExecResult[];
export declare function upsertGroupReadCursor(db: Database, cursorJSON: string): QueryExecResult[];
export declare function getMinReadSeqFromCursors(db: Database, conversationID: string): QueryExecResult[];
