import { Database, QueryExecResult } from '@jlongster/sql.js';
export declare type LocalTableMaster = {
    [key: string]: any;
};
export declare function getExistedTables(db: Database): QueryExecResult[];
