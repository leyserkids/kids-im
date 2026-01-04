import Emitter from '../utils/emitter';
import { AccessFriendApplicationParams, AccessGroupApplicationParams, AccessMessageParams, AddFriendParams, AdvancedMsgParams, AdvancedQuoteMsgParams, AtMsgParams, ChangeGroupMemberMuteParams, ChangeGroupMuteParams, CreateGroupParams, CustomMsgParams, CustomSignalParams, FaceMessageParams, FileMsgParamsByURL, FindMessageParams, GetAdvancedHistoryMsgParams, GetGroupMemberByTimeParams, GetGroupMemberParams, GetGroupMessageReaderParams, GetGroupMessageReadMemberListParams, GetHistoryMsgParams, GetOneConversationParams, ImageMsgParamsByURL, InitAndLoginConfig, InsertGroupMsgParams, InsertSingleMsgParams, AccessToGroupParams, SetConversationRecvOptParams, JoinGroupParams, LocationMsgParams, UpdateMemberInfoParams, MergerMsgParams, PartialUserItem, SetConversationPinParams, QuoteMsgParams, RemarkFriendParams, RtcActionParams, SearchFriendParams, SearchGroupMemberParams, SearchGroupParams, SearchLocalParams, SendGroupReadReceiptParams, SendMsgParams, SetBurnDurationParams, SetConversationMsgDestructParams, SetConversationMsgDestructTimeParams, SetConversationDraftParams, SetGroupRoleParams, SetGroupVerificationParams, SetMemberPermissionParams, SetMessageLocalExParams, SetConversationPrivateStateParams, SignalingInviteParams, SoundMsgParamsByURL, SplitConversationParams, TransferGroupParams, TypingUpdateParams, UploadFileParams, VideoMsgParamsByURL, SetGroupMemberNickParams, WasmPathConfig, PinFriendParams, SetFriendExParams, SetConversationExParams, AddBlackParams, OffsetParams, UpdateFriendsParams, SetConversationParams, GetSpecifiedFriendsParams, ChangeInputStatesParams, GetInputstatesParams, FetchSurroundingParams } from '../types/params';
import { AdvancedGetMessageResult, BlackUserItem, CallingRoomData, CardElem, ConversationItem, FriendApplicationItem, FriendshipInfo, FriendUserItem, GroupApplicationItem, GroupItem, GroupMemberItem, GroupMessageReceipt, MessageItem, OfflinePush, PublicUserItem, RtcInvite, RtcInviteResults, SearchedFriendsInfo, SearchMessageResult, SelfUserInfo, UserOnlineState, WsResponse } from '../types/entity';
import { LoginStatus, MessageReceiveOptType, Platform } from '../types/enum';
declare class SDK extends Emitter {
    private wasmInitializedPromise;
    private goExitPromise;
    private goExisted;
    private tryParse;
    private isLogStandardOutput;
    constructor(url?: string, debug?: boolean);
    _logWrap(...args: any[]): void;
    _invoker<T>(functionName: string, func: (...args: any[]) => Promise<any>, args: any[], processor?: (data: string) => string): Promise<WsResponse<T>>;
    login: (params: InitAndLoginConfig, operationID?: string) => Promise<any>;
    logout: <T>(operationID?: string) => Promise<WsResponse<T>>;
    getAllConversationList: (operationID?: string) => Promise<WsResponse<ConversationItem[]>>;
    getOneConversation: (params: GetOneConversationParams, operationID?: string) => Promise<WsResponse<ConversationItem>>;
    getAdvancedHistoryMessageList: (params: GetAdvancedHistoryMsgParams, operationID?: string) => Promise<WsResponse<AdvancedGetMessageResult>>;
    getAdvancedHistoryMessageListReverse: (params: GetAdvancedHistoryMsgParams, operationID?: string) => Promise<WsResponse<AdvancedGetMessageResult>>;
    fetchSurroundingMessages: (params: FetchSurroundingParams, operationID?: string) => Promise<WsResponse<{
        messageList: MessageItem[];
    }>>;
    getSpecifiedGroupsInfo: (params: string[], operationID?: string) => Promise<WsResponse<GroupItem[]>>;
    deleteConversationAndDeleteAllMsg: <T>(conversationID: string, operationID?: string) => Promise<WsResponse<T>>;
    markConversationMessageAsRead: <T>(data: string, operationID?: string) => Promise<WsResponse<T>>;
    sendGroupMessageReadReceipt: <T>(params: SendGroupReadReceiptParams, operationID?: string) => Promise<WsResponse<T>>;
    getGroupMessageReaderList: (params: GetGroupMessageReaderParams, operationID?: string) => Promise<WsResponse<GroupMemberItem[]>>;
    getGroupMessageReadMemberList: (params: GetGroupMessageReadMemberListParams, operationID?: string) => Promise<WsResponse<GroupMessageReceipt[]>>;
    getGroupMemberList: (params: GetGroupMemberParams, operationID?: string) => Promise<WsResponse<GroupMemberItem[]>>;
    createTextMessage: (text: string, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createImageMessageByURL: (params: ImageMsgParamsByURL, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createImageMessageByFile: (params: ImageMsgParamsByURL & {
        file: File;
    }, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createCustomMessage: (params: CustomMsgParams, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createQuoteMessage: (params: QuoteMsgParams, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createAdvancedQuoteMessage: (params: AdvancedQuoteMsgParams, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createAdvancedTextMessage: (params: AdvancedMsgParams, operationID?: string) => Promise<WsResponse<MessageItem>>;
    sendMessage: (params: SendMsgParams, operationID?: string) => Promise<WsResponse<MessageItem>>;
    sendMessageNotOss: (params: SendMsgParams, operationID?: string) => Promise<WsResponse<MessageItem>>;
    setMessageLocalEx: <T>(params: SetMessageLocalExParams, operationID?: string) => Promise<WsResponse<T>>;
    exportDB(operationID?: string): Promise<WsResponse<unknown>>;
    getHistoryMessageListReverse: (params: GetHistoryMsgParams, operationID?: string) => Promise<WsResponse<AdvancedGetMessageResult>>;
    revokeMessage: <T>(data: AccessMessageParams, operationID?: string) => Promise<WsResponse<T>>;
    setConversation: <T>(params: SetConversationParams, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use setConversation instead.
     */
    setConversationPrivateChat: <T>(params: SetConversationPrivateStateParams, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use setConversation instead.
     */
    setConversationBurnDuration: <T>(params: SetBurnDurationParams, operationID?: string) => Promise<WsResponse<T>>;
    getLoginStatus: (operationID?: string) => Promise<WsResponse<LoginStatus>>;
    setAppBackgroundStatus: <T>(data: boolean, operationID?: string) => Promise<WsResponse<T>>;
    networkStatusChanged: <T>(operationID?: string) => Promise<WsResponse<T>>;
    getLoginUserID: (operationID?: string) => Promise<WsResponse<string>>;
    getSelfUserInfo: (operationID?: string) => Promise<WsResponse<SelfUserInfo>>;
    getUsersInfo: (data: string[], operationID?: string) => Promise<WsResponse<PublicUserItem[]>>;
    /**
     * @deprecated Use setSelfInfo instead.
     */
    SetSelfInfoEx: <T>(data: PartialUserItem, operationID?: string) => Promise<WsResponse<T>>;
    setSelfInfo: <T>(data: PartialUserItem, operationID?: string) => Promise<WsResponse<T>>;
    createTextAtMessage: (data: AtMsgParams, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createSoundMessageByURL: (data: SoundMsgParamsByURL, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createSoundMessageByFile: (data: SoundMsgParamsByURL & {
        file: File;
    }, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createVideoMessageByURL: (data: VideoMsgParamsByURL, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createVideoMessageByFile: (data: VideoMsgParamsByURL & {
        videoFile: File;
        snapshotFile: File;
    }, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createFileMessageByURL: (data: FileMsgParamsByURL, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createFileMessageByFile: (data: FileMsgParamsByURL & {
        file: File;
    }, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createMergerMessage: (data: MergerMsgParams, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createForwardMessage: (data: MessageItem, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createFaceMessage: (data: FaceMessageParams, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createLocationMessage: (data: LocationMsgParams, operationID?: string) => Promise<WsResponse<MessageItem>>;
    createCardMessage: (data: CardElem, operationID?: string) => Promise<WsResponse<MessageItem>>;
    deleteMessageFromLocalStorage: <T>(data: AccessMessageParams, operationID?: string) => Promise<WsResponse<T>>;
    deleteMessage: <T>(data: AccessMessageParams, operationID?: string) => Promise<WsResponse<T>>;
    deleteAllConversationFromLocal: <T>(operationID?: string) => Promise<WsResponse<T>>;
    deleteAllMsgFromLocal: <T>(operationID?: string) => Promise<WsResponse<T>>;
    deleteAllMsgFromLocalAndSvr: <T>(operationID?: string) => Promise<WsResponse<T>>;
    insertSingleMessageToLocalStorage: <T>(data: InsertSingleMsgParams, operationID?: string) => Promise<WsResponse<T>>;
    insertGroupMessageToLocalStorage: <T>(data: InsertGroupMsgParams, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use changeInputStates instead.
     */
    typingStatusUpdate: <T>(data: TypingUpdateParams, operationID?: string) => Promise<WsResponse<T>>;
    changeInputStates: (data: ChangeInputStatesParams, operationID?: string) => Promise<WsResponse<void>>;
    getInputstates: (data: GetInputstatesParams, operationID?: string) => Promise<WsResponse<Platform[]>>;
    clearConversationAndDeleteAllMsg: <T>(data: string, operationID?: string) => Promise<WsResponse<T>>;
    hideConversation: <T>(data: string, operationID?: string) => Promise<WsResponse<T>>;
    getConversationListSplit: (data: SplitConversationParams, operationID?: string) => Promise<WsResponse<ConversationItem[]>>;
    /**
     * @deprecated Use setConversation instead.
     */
    setConversationEx: (data: SetConversationExParams, operationID?: string) => Promise<WsResponse<ConversationItem[]>>;
    getConversationIDBySessionType: (data: GetOneConversationParams, operationID?: string) => Promise<WsResponse<string>>;
    getMultipleConversation: (data: string[], operationID?: string) => Promise<WsResponse<ConversationItem[]>>;
    deleteConversation: <T>(data: string, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use setConversation instead.
     */
    setConversationDraft: <T>(data: SetConversationDraftParams, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use setConversation instead.
     */
    pinConversation: <T>(data: SetConversationPinParams, operationID?: string) => Promise<WsResponse<T>>;
    getTotalUnreadMsgCount: (operationID?: string) => Promise<WsResponse<number>>;
    getConversationRecvMessageOpt: (data: string[], operationID?: string) => Promise<WsResponse<ConversationItem[]>>;
    /**
     * @deprecated Use setConversation instead.
     */
    setConversationRecvMessageOpt: <T>(data: SetConversationRecvOptParams, operationID?: string) => Promise<WsResponse<T>>;
    searchLocalMessages: (data: SearchLocalParams, operationID?: string) => Promise<WsResponse<SearchMessageResult>>;
    addFriend: <T>(data: AddFriendParams, operationID?: string) => Promise<WsResponse<T>>;
    searchFriends: (data: SearchFriendParams, operationID?: string) => Promise<WsResponse<SearchedFriendsInfo[]>>;
    getSpecifiedFriendsInfo: (data: GetSpecifiedFriendsParams, operationID?: string) => Promise<WsResponse<FriendUserItem[]>>;
    getFriendApplicationListAsRecipient: (operationID?: string) => Promise<WsResponse<FriendApplicationItem[]>>;
    getFriendApplicationListAsApplicant: (operationID?: string) => Promise<WsResponse<FriendApplicationItem[]>>;
    getFriendList: (filterBlack?: boolean, operationID?: string) => Promise<WsResponse<FriendUserItem[]>>;
    getFriendListPage: (data: OffsetParams & {
        filterBlack?: boolean;
    }, operationID?: string) => Promise<WsResponse<FriendUserItem[]>>;
    updateFriends: <T>(data: UpdateFriendsParams, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use updateFriends instead.
     */
    setFriendRemark: <T>(data: RemarkFriendParams, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use updateFriends instead.
     */
    pinFriends: <T>(data: PinFriendParams, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use updateFriends instead.
     */
    setFriendsEx: <T>(data: SetFriendExParams, operationID?: string) => Promise<WsResponse<T>>;
    checkFriend: (data: string[], operationID?: string) => Promise<WsResponse<FriendshipInfo[]>>;
    acceptFriendApplication: <T>(data: AccessFriendApplicationParams, operationID?: string) => Promise<WsResponse<T>>;
    refuseFriendApplication: <T>(data: AccessFriendApplicationParams, operationID?: string) => Promise<WsResponse<T>>;
    deleteFriend: <T>(data: string, operationID?: string) => Promise<WsResponse<T>>;
    addBlack: <T>(data: AddBlackParams, operationID?: string) => Promise<WsResponse<T>>;
    removeBlack: <T>(data: string, operationID?: string) => Promise<WsResponse<T>>;
    getBlackList: (operationID?: string) => Promise<WsResponse<BlackUserItem[]>>;
    inviteUserToGroup: <T>(data: AccessToGroupParams, operationID?: string) => Promise<WsResponse<T>>;
    kickGroupMember: <T>(data: AccessToGroupParams, operationID?: string) => Promise<WsResponse<T>>;
    isJoinGroup: <T>(data: string, operationID?: string) => Promise<WsResponse<T>>;
    getSpecifiedGroupMembersInfo: (data: Omit<AccessToGroupParams, 'reason'>, operationID?: string) => Promise<WsResponse<GroupMemberItem[]>>;
    getUsersInGroup: (data: Omit<AccessToGroupParams, 'reason'>, operationID?: string) => Promise<WsResponse<string[]>>;
    getGroupMemberListByJoinTimeFilter: (data: GetGroupMemberByTimeParams, operationID?: string) => Promise<WsResponse<GroupMemberItem[]>>;
    searchGroupMembers: (data: SearchGroupMemberParams, operationID?: string) => Promise<WsResponse<GroupMemberItem[]>>;
    /**
     * @deprecated Use setGroupInfo instead.
     */
    setGroupApplyMemberFriend: <T>(data: SetMemberPermissionParams, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use setGroupInfo instead.
     */
    setGroupLookMemberInfo: <T>(data: SetMemberPermissionParams, operationID?: string) => Promise<WsResponse<T>>;
    getJoinedGroupList: (operationID?: string) => Promise<WsResponse<GroupItem[]>>;
    getJoinedGroupListPage: (data: OffsetParams, operationID?: string) => Promise<WsResponse<GroupItem[]>>;
    createGroup: (data: CreateGroupParams, operationID?: string) => Promise<WsResponse<GroupItem>>;
    setGroupInfo: <T>(data: Partial<GroupItem> & {
        groupID: string;
    }, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use setGroupMemberInfo instead.
     */
    setGroupMemberNickname: <T>(data: SetGroupMemberNickParams, operationID?: string) => Promise<WsResponse<T>>;
    setGroupMemberInfo: <T>(data: UpdateMemberInfoParams, operationID?: string) => Promise<WsResponse<T>>;
    joinGroup: <T>(data: JoinGroupParams, operationID?: string) => Promise<WsResponse<T>>;
    searchGroups: (data: SearchGroupParams, operationID?: string) => Promise<WsResponse<GroupItem[]>>;
    quitGroup: <T>(data: string, operationID?: string) => Promise<WsResponse<T>>;
    dismissGroup: <T>(data: string, operationID?: string) => Promise<WsResponse<T>>;
    changeGroupMute: <T>(data: ChangeGroupMuteParams, operationID?: string) => Promise<WsResponse<T>>;
    changeGroupMemberMute: <T>(data: ChangeGroupMemberMuteParams, operationID?: string) => Promise<WsResponse<T>>;
    transferGroupOwner: <T>(data: TransferGroupParams, operationID?: string) => Promise<WsResponse<T>>;
    getGroupApplicationListAsApplicant: (operationID?: string) => Promise<WsResponse<GroupApplicationItem[]>>;
    getGroupApplicationListAsRecipient: (operationID?: string) => Promise<WsResponse<GroupApplicationItem[]>>;
    acceptGroupApplication: <T>(data: AccessGroupApplicationParams, operationID?: string) => Promise<WsResponse<T>>;
    refuseGroupApplication: <T>(data: AccessGroupApplicationParams, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use setConversation instead.
     */
    resetConversationGroupAtType: <T>(data: string, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use setGroupMemberInfo instead.
     */
    setGroupMemberRoleLevel: <T>(data: SetGroupRoleParams, operationID?: string) => Promise<WsResponse<T>>;
    /**
     * @deprecated Use setGroupInfo instead.
     */
    setGroupVerification: <T>(data: SetGroupVerificationParams, operationID?: string) => Promise<WsResponse<T>>;
    getGroupMemberOwnerAndAdmin: (data: string, operationID?: string) => Promise<WsResponse<GroupMemberItem[]>>;
    /**
     * @deprecated Use setSelfInfo instead.
     */
    setGlobalRecvMessageOpt: <T>(opt: MessageReceiveOptType, operationID?: string) => Promise<WsResponse<T>>;
    findMessageList: (data: FindMessageParams[], operationID?: string) => Promise<WsResponse<SearchMessageResult>>;
    uploadFile: (data: UploadFileParams, operationID?: string) => Promise<WsResponse<{
        url: string;
    }>>;
    subscribeUsersStatus: (data: string[], operationID?: string) => Promise<WsResponse<UserOnlineState[]>>;
    unsubscribeUsersStatus: (data: string[], operationID?: string) => Promise<WsResponse<UserOnlineState[]>>;
    subscribeConversationReadState: (conversationID: string) => Promise<WsResponse<number>>;
    unsubscribeConversationReadState: (conversationID: string) => Promise<WsResponse<void>>;
    getUserStatus: (operationID?: string) => Promise<WsResponse<UserOnlineState[]>>;
    getSubscribeUsersStatus: (operationID?: string) => Promise<WsResponse<UserOnlineState[]>>;
    signalingInvite: (data: SignalingInviteParams, operationID?: string) => Promise<WsResponse<RtcInviteResults>>;
    signalingInviteInGroup: (data: SignalingInviteParams, operationID?: string) => Promise<WsResponse<RtcInviteResults>>;
    signalingAccept: (data: RtcActionParams, operationID?: string) => Promise<WsResponse<RtcInviteResults>>;
    signalingReject: <T>(data: RtcActionParams, operationID?: string) => Promise<WsResponse<T>>;
    signalingCancel: <T>(data: RtcActionParams, operationID?: string) => Promise<WsResponse<T>>;
    signalingHungUp: <T>(data: RtcActionParams, operationID?: string) => Promise<WsResponse<T>>;
    signalingGetRoomByGroupID: (groupID: string, operationID?: string) => Promise<WsResponse<CallingRoomData>>;
    signalingGetTokenByRoomID: (roomID: string, operationID?: string) => Promise<WsResponse<RtcInviteResults>>;
    getSignalingInvitationInfoStartApp: (operationID?: string) => Promise<WsResponse<{
        invitation: RtcInvite | null;
        offlinePushInfo: OfflinePush;
    }>>;
    signalingSendCustomSignal: <T>(data: CustomSignalParams, operationID?: string) => Promise<WsResponse<T>>;
    setConversationIsMsgDestruct: <T>(data: SetConversationMsgDestructParams, operationID?: string) => Promise<WsResponse<T>>;
    setConversationMsgDestructTime: <T>(data: SetConversationMsgDestructTimeParams, operationID?: string) => Promise<WsResponse<T>>;
    fileMapSet: (uuid: string, file: File) => Promise<any>;
}
export declare function getSDK(config?: WasmPathConfig): SDK;
export {};
