import { MessageEntity, OfflinePush, PicBaseInfo, AtUsersInfoItem, MessageItem, SelfUserInfo, RtcInvite, GroupItem } from './entity';
import { AllowType, GroupJoinSource, GroupVerificationType, MessageType, MessageReceiveOptType, GroupMemberRole, GroupMemberFilter, LogLevel, GroupMessageReaderFilter, GroupAtType, ViewType } from './enum';
export declare type WasmPathConfig = {
    coreWasmPath?: string;
    sqlWasmPath?: string;
    debug?: boolean;
};
export declare type InitAndLoginConfig = {
    userID: string;
    token: string;
    platformID: number;
    apiAddr: string;
    wsAddr: string;
    logLevel?: LogLevel;
    isLogStandardOutput?: boolean;
    isExternalExtensions?: boolean;
    tryParse?: boolean;
};
export declare type GetOneConversationParams = {
    sourceID: string;
    sessionType: number;
};
export declare type GetAdvancedHistoryMsgParams = {
    count: number;
    viewType: ViewType;
    startClientMsgID: string;
    conversationID: string;
};
export declare type FetchSurroundingParams = {
    startMessage: MessageItem;
    viewType: ViewType;
    before: number;
    after: number;
};
export declare type GetHistoryMsgParams = {
    userID: string;
    groupID: string;
    count: number;
    startClientMsgID: string;
    conversationID?: string;
};
export declare type SendGroupReadReceiptParams = {
    conversationID: string;
    clientMsgIDList: string[];
};
export declare type GetGroupMessageReaderParams = {
    conversationID: string;
    clientMsgID: string;
    filter: GroupMessageReaderFilter;
    offset: number;
    count: number;
};
export declare type GetGroupMessageReadMemberListParams = {
    conversationID: string;
    seq: number;
};
export declare type GetGroupMemberParams = {
    groupID: string;
    filter: GroupMemberFilter;
    offset: number;
    count: number;
};
export declare type SendMsgParams = {
    recvID: string;
    groupID: string;
    offlinePushInfo?: OfflinePush;
    message: MessageItem;
    isOnlineOnly?: boolean;
};
export declare type SetMessageLocalExParams = {
    conversationID: string;
    clientMsgID: string;
    localEx: string;
};
export declare type ImageMsgParamsByURL = {
    sourcePicture: PicBaseInfo;
    bigPicture: PicBaseInfo;
    snapshotPicture: PicBaseInfo;
    sourcePath: string;
};
export declare type VideoMsgParamsByURL = {
    videoPath: string;
    duration: number;
    videoType: string;
    snapshotPath: string;
    videoUUID: string;
    videoUrl: string;
    videoSize: number;
    snapshotUUID: string;
    snapshotSize: number;
    snapshotUrl: string;
    snapshotWidth: number;
    snapshotHeight: number;
    snapShotType?: string;
};
export declare type VideoMsgParamsByFullPath = {
    videoFullPath: string;
    videoType: string;
    duration: number;
    snapshotFullPath: string;
};
export declare type CustomMsgParams = {
    data: string;
    extension: string;
    description: string;
};
export declare type QuoteMsgParams = {
    text: string;
    message: string;
};
export declare type AdvancedQuoteMsgParams = {
    text: string;
    message: MessageItem;
    messageEntityList?: MessageEntity[];
};
export declare type AdvancedMsgParams = {
    text: string;
    messageEntityList?: MessageEntity[];
};
export declare type SetConversationParams = {
    conversationID: string;
    recvMsgOpt?: MessageReceiveOptType;
    groupAtType?: GroupAtType;
    burnDuration?: number;
    msgDestructTime?: number;
    isPinned?: boolean;
    isPrivateChat?: boolean;
    isMsgDestruct?: boolean;
    ex?: string;
};
export declare type SetConversationPrivateStateParams = {
    conversationID: string;
    isPrivate: boolean;
};
export declare type SplitConversationParams = {
    offset: number;
    count: number;
};
export declare type SetConversationDraftParams = {
    conversationID: string;
    draftText: string;
};
export declare type SetConversationPinParams = {
    conversationID: string;
    isPinned: boolean;
};
export declare type JoinGroupParams = {
    groupID: string;
    reqMsg: string;
    joinSource: GroupJoinSource;
    ex?: string;
};
export declare type SearchGroupParams = {
    keywordList: string[];
    isSearchGroupID: boolean;
    isSearchGroupName: boolean;
};
export declare type ChangeGroupMuteParams = {
    groupID: string;
    isMute: boolean;
};
export declare type ChangeGroupMemberMuteParams = {
    groupID: string;
    userID: string;
    mutedSeconds: number;
};
export declare type TransferGroupParams = {
    groupID: string;
    newOwnerUserID: string;
};
export declare type AccessGroupApplicationParams = {
    groupID: string;
    fromUserID: string;
    handleMsg: string;
};
export declare type SetGroupRoleParams = {
    groupID: string;
    userID: string;
    roleLevel: GroupMemberRole;
};
export declare type SetGroupVerificationParams = {
    verification: GroupVerificationType;
    groupID: string;
};
export declare type SetBurnDurationParams = {
    conversationID: string;
    burnDuration: number;
};
export declare type AtMsgParams = {
    text: string;
    atUserIDList: string[];
    atUsersInfo?: AtUsersInfoItem[];
    message?: MessageItem;
};
export declare type SoundMsgParamsByURL = {
    uuid: string;
    soundPath: string;
    sourceUrl: string;
    dataSize: number;
    duration: number;
    soundType?: string;
};
export declare type FileMsgParamsByURL = {
    filePath: string;
    fileName: string;
    uuid: string;
    sourceUrl: string;
    fileSize: number;
    fileType?: string;
};
export declare type FileMsgParamsByFullPath = {
    fileFullPath: string;
    fileName: string;
};
export declare type SoundMsgParamsByFullPath = {
    soundPath: string;
    duration: number;
};
export declare type MergerMsgParams = {
    messageList: MessageItem[];
    title: string;
    summaryList: string[];
};
export declare type FaceMessageParams = {
    index: number;
    data: string;
};
export declare type LocationMsgParams = {
    description: string;
    longitude: number;
    latitude: number;
};
export declare type InsertSingleMsgParams = {
    message: MessageItem;
    recvID: string;
    sendID: string;
};
export declare type InsertGroupMsgParams = {
    message: MessageItem;
    groupID: string;
    sendID: string;
};
export declare type AccessMessageParams = {
    conversationID: string;
    clientMsgID: string;
};
export declare type TypingUpdateParams = {
    recvID: string;
    msgTip: string;
};
export declare type ChangeInputStatesParams = {
    conversationID: string;
    focus: boolean;
};
export declare type GetInputstatesParams = {
    conversationID: string;
    userID: string;
};
export declare type SetConversationExParams = {
    conversationID: string;
    ex: string;
};
export declare type SetConversationRecvOptParams = {
    conversationID: string;
    opt: MessageReceiveOptType;
};
export declare type SearchLocalParams = {
    conversationID: string;
    keywordList: string[];
    keywordListMatchType?: number;
    senderUserIDList?: string[];
    messageTypeList?: MessageType[];
    searchTimePosition?: number;
    searchTimePeriod?: number;
    pageIndex?: number;
    count?: number;
};
export declare type AddFriendParams = {
    toUserID: string;
    reqMsg: string;
};
export declare type SearchFriendParams = {
    keywordList: string[];
    isSearchUserID: boolean;
    isSearchNickname: boolean;
    isSearchRemark: boolean;
};
export declare type GetSpecifiedFriendsParams = {
    friendUserIDList: string[];
    filterBlack?: boolean;
};
export declare type UpdateFriendsParams = {
    friendUserIDs: string[];
    isPinned?: boolean;
    remark?: string;
    ex?: string;
};
export declare type RemarkFriendParams = {
    toUserID: string;
    remark: string;
};
export declare type PinFriendParams = {
    toUserIDs: string[];
    isPinned: boolean;
};
export declare type SetFriendExParams = {
    toUserIDs: string[];
    ex: string;
};
export declare type AccessFriendApplicationParams = {
    toUserID: string;
    handleMsg: string;
};
export declare type AddBlackParams = {
    toUserID: string;
    ex?: string;
};
export declare type AccessToGroupParams = {
    groupID: string;
    reason: string;
    userIDList: string[];
};
export declare type GetGroupMemberByTimeParams = {
    groupID: string;
    filterUserIDList: string[];
    offset: number;
    count: number;
    joinTimeBegin: number;
    joinTimeEnd: number;
};
export declare type SearchGroupMemberParams = {
    groupID: string;
    keywordList: string[];
    isSearchUserID: boolean;
    isSearchMemberNickname: boolean;
    offset: number;
    count: number;
};
export declare type SetMemberPermissionParams = {
    rule: AllowType;
    groupID: string;
};
export declare type OffsetParams = {
    offset: number;
    count: number;
};
export declare type CreateGroupParams = {
    memberUserIDs: string[];
    groupInfo: Partial<GroupItem>;
    adminUserIDs?: string[];
    ownerUserID?: string;
};
export declare type SetGroupMemberNickParams = {
    groupID: string;
    userID: string;
    groupMemberNickname: string;
};
export declare type UpdateMemberInfoParams = {
    groupID: string;
    userID: string;
    nickname?: string;
    faceURL?: string;
    roleLevel?: GroupMemberRole;
    ex?: string;
};
export declare type FindMessageParams = {
    conversationID: string;
    clientMsgIDList: string[];
};
export declare type UploadFileParams = {
    name: string;
    contentType: string;
    uuid: string;
    file?: File;
    filepath?: string;
    cause?: string;
};
export declare type PartialUserItem = Partial<SelfUserInfo>;
export declare type SignalingInviteParams = {
    invitation: RtcInvite;
    offlinePushInfo?: OfflinePush;
};
export declare type RtcActionParams = {
    opUserID: string;
    invitation: RtcInvite;
};
export declare type CustomSignalParams = {
    roomID: string;
    customInfo: string;
};
export declare type SetConversationMsgDestructParams = {
    conversationID: string;
    isMsgDestruct: boolean;
};
export declare type SetConversationMsgDestructTimeParams = {
    conversationID: string;
    msgDestructTime: number;
};
