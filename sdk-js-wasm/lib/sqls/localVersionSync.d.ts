import { Database, QueryExecResult } from '@jlongster/sql.js';
export declare type LocalVersionSync = {
    [key: string]: any;
};
export declare function localVersionSyncs(db: Database): QueryExecResult[];
export declare function getVersionSync(db: Database, tableName: string, entityID: string): QueryExecResult[];
export declare function insertVersionSync(db: Database, localVersionSync: LocalVersionSync): QueryExecResult[];
export declare function updateVersionSync(db: Database, oldTable: string, oldEntityID: string, localVersionSync: LocalVersionSync): QueryExecResult[];
export declare function deleteVersionSync(db: Database, tableName: string, entityID: string): QueryExecResult[];
