import { DatabaseErrorCode } from '@/constant';
import {
  insertGroupReadCursor as databaseInsertGroupReadCursor,
  getGroupReadCursor as databaseGetGroupReadCursor,
  getGroupReadCursorsByConversationID as databaseGetGroupReadCursorsByConversationID,
  updateGroupReadCursor as databaseUpdateGroupReadCursor,
  deleteGroupReadCursor as databaseDeleteGroupReadCursor,
  deleteGroupReadCursorsByConversationID as databaseDeleteGroupReadCursorsByConversationID,
  upsertGroupReadCursor as databaseUpsertGroupReadCursor,
  getMinReadSeqFromCursors as databaseGetMinReadSeqFromCursors,
  getGroupReadState as databaseGetGroupReadState,
  upsertGroupReadState as databaseUpsertGroupReadState,
  updateGroupReadStateMinSeq as databaseUpdateGroupReadStateMinSeq,
  deleteGroupReadState as databaseDeleteGroupReadState,
} from '@/sqls';
import { converSqlExecResult, formatResponse } from '@/utils';
import { getInstance } from './instance';

export async function insertGroupReadCursor(
  cursorJSON: string
): Promise<string> {
  try {
    const db = await getInstance();
    databaseInsertGroupReadCursor(db, cursorJSON);
    return formatResponse('');
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function getGroupReadCursor(
  conversationID: string,
  userID: string
): Promise<string> {
  try {
    const db = await getInstance();
    const execResult = databaseGetGroupReadCursor(db, conversationID, userID);

    if (execResult.length === 0 || !execResult[0].values.length) {
      return formatResponse(
        '',
        DatabaseErrorCode.ErrorNoRecord,
        `no cursor for conversation ${conversationID} and user ${userID}`
      );
    }

    return formatResponse(converSqlExecResult(execResult[0], 'CamelCase')[0]);
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function getGroupReadCursorsByConversationID(
  conversationID: string
): Promise<string> {
  try {
    const db = await getInstance();
    const execResult = databaseGetGroupReadCursorsByConversationID(
      db,
      conversationID
    );

    return formatResponse(converSqlExecResult(execResult[0], 'CamelCase'));
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function updateGroupReadCursor(
  conversationID: string,
  userID: string,
  maxReadSeq: number
): Promise<string> {
  try {
    const db = await getInstance();
    databaseUpdateGroupReadCursor(db, conversationID, userID, maxReadSeq);
    return formatResponse('');
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function deleteGroupReadCursor(
  conversationID: string,
  userID: string
): Promise<string> {
  try {
    const db = await getInstance();
    databaseDeleteGroupReadCursor(db, conversationID, userID);
    return formatResponse('');
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function deleteGroupReadCursorsByConversationID(
  conversationID: string
): Promise<string> {
  try {
    const db = await getInstance();
    databaseDeleteGroupReadCursorsByConversationID(db, conversationID);
    return formatResponse('');
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function upsertGroupReadCursor(
  cursorJSON: string
): Promise<string> {
  try {
    const db = await getInstance();
    databaseUpsertGroupReadCursor(db, cursorJSON);
    return formatResponse('');
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function getMinReadSeqFromCursors(
  conversationID: string
): Promise<string> {
  try {
    const db = await getInstance();
    const execResult = databaseGetMinReadSeqFromCursors(db, conversationID);

    if (execResult.length === 0 || !execResult[0].values.length) {
      return formatResponse(0);
    }

    const minSeq = execResult[0].values[0][0] as number;
    return formatResponse(minSeq || 0);
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function getGroupReadState(
  conversationID: string
): Promise<string> {
  try {
    const db = await getInstance();
    const execResult = databaseGetGroupReadState(db, conversationID);

    if (execResult.length === 0 || !execResult[0].values.length) {
      return formatResponse(
        '',
        DatabaseErrorCode.ErrorNoRecord,
        `no read state for conversation ${conversationID}`
      );
    }

    return formatResponse(converSqlExecResult(execResult[0], 'CamelCase')[0]);
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function upsertGroupReadState(stateJSON: string): Promise<string> {
  try {
    const db = await getInstance();
    databaseUpsertGroupReadState(db, stateJSON);
    return formatResponse('');
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function updateGroupReadStateMinSeq(
  conversationID: string,
  minReadSeq: number
): Promise<string> {
  try {
    const db = await getInstance();
    databaseUpdateGroupReadStateMinSeq(db, conversationID, minReadSeq);
    return formatResponse('');
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function deleteGroupReadState(
  conversationID: string
): Promise<string> {
  try {
    const db = await getInstance();
    databaseDeleteGroupReadState(db, conversationID);
    return formatResponse('');
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}
