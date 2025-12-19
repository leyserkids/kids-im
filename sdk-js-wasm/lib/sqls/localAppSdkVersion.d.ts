import { Database, QueryExecResult } from '@jlongster/sql.js';
export declare type LocalAppSDKVersion = {
    [key: string]: any;
};
export declare function localAppSDKVersions(db: Database): QueryExecResult[];
export declare function getAppSDKVersion(db: Database): QueryExecResult[];
export declare function insertAppSDKVersion(db: Database, localAppSDKVersion: LocalAppSDKVersion): QueryExecResult[];
export declare function updateAppSDKVersion(db: Database, oldVersion: string, localAppSDKVersion: LocalAppSDKVersion): QueryExecResult[];
