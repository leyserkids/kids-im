import { Database, QueryExecResult } from '@jlongster/sql.js';
export declare type LocalGroupReadCursorState = {
    conversationID: string;
    cursorVersion: number;
};
export declare function localGroupReadCursorState(db: Database): QueryExecResult[];
export declare function insertGroupReadCursorState(db: Database, stateJSON: string): QueryExecResult[];
export declare function getGroupReadCursorState(db: Database, conversationID: string): QueryExecResult[];
export declare function deleteGroupReadCursorState(db: Database, conversationID: string): QueryExecResult[];
export declare function incrementGroupReadCursorVersion(db: Database, conversationID: string): QueryExecResult[];
