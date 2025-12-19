export declare function setNotificationSeq(conversationID: string, seq: number): Promise<string>;
export declare function getNotificationAllSeqs(): Promise<string>;
export declare function batchInsertNotificationSeq(local_notification_seqs: string): Promise<string>;
