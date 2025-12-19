import { DatabaseErrorCode } from '@/constant';
import {
  insertGroupReadCursor as databaseInsertGroupReadCursor,
  getGroupReadCursor as databaseGetGroupReadCursor,
  getGroupReadCursorsByConversationID as databaseGetGroupReadCursorsByConversationID,
  updateGroupReadCursor as databaseUpdateGroupReadCursor,
  deleteGroupReadCursor as databaseDeleteGroupReadCursor,
  deleteGroupReadCursorsByConversationID as databaseDeleteGroupReadCursorsByConversationID,
  insertGroupReadCursorState as databaseInsertGroupReadCursorState,
  getGroupReadCursorState as databaseGetGroupReadCursorState,
  deleteGroupReadCursorState as databaseDeleteGroupReadCursorState,
  incrementGroupReadCursorVersion as databaseIncrementGroupReadCursorVersion,
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

export async function insertGroupReadCursorState(
  stateJSON: string
): Promise<string> {
  try {
    const db = await getInstance();
    databaseInsertGroupReadCursorState(db, stateJSON);
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

export async function getGroupReadCursorState(
  conversationID: string
): Promise<string> {
  try {
    const db = await getInstance();
    const execResult = databaseGetGroupReadCursorState(db, conversationID);

    if (execResult.length === 0 || !execResult[0].values.length) {
      return formatResponse(
        '',
        DatabaseErrorCode.ErrorNoRecord,
        `no cursor state for conversation ${conversationID}`
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

export async function deleteGroupReadCursorState(
  conversationID: string
): Promise<string> {
  try {
    const db = await getInstance();
    databaseDeleteGroupReadCursorState(db, conversationID);
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

export async function incrementGroupReadCursorVersion(
  conversationID: string
): Promise<string> {
  try {
    const db = await getInstance();
    databaseIncrementGroupReadCursorVersion(db, conversationID);
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
