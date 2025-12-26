import { DatabaseErrorCode } from '@/constant';
import {
  insertReadCursor as databaseInsertReadCursor,
  getReadCursor as databaseGetReadCursor,
  getReadCursorsByConversationID as databaseGetReadCursorsByConversationID,
  updateReadCursor as databaseUpdateReadCursor,
  deleteReadCursor as databaseDeleteReadCursor,
  deleteReadCursorsByConversationID as databaseDeleteReadCursorsByConversationID,
  upsertReadCursor as databaseUpsertReadCursor,
  getAllReadSeqFromCursors as databaseGetAllReadSeqFromCursors,
  getReadStateDB as databaseGetReadState,
  upsertReadStateDB as databaseUpsertReadState,
  updateReadStateAllReadSeqDB as databaseUpdateReadStateAllReadSeq,
  deleteReadStateDB as databaseDeleteReadState,
} from '@/sqls';
import { converSqlExecResult, formatResponse } from '@/utils';
import { getInstance } from './instance';

export async function insertReadCursor(cursorJSON: string): Promise<string> {
  try {
    const db = await getInstance();
    databaseInsertReadCursor(db, cursorJSON);
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

export async function getReadCursor(
  conversationID: string,
  userID: string
): Promise<string> {
  try {
    const db = await getInstance();
    const execResult = databaseGetReadCursor(db, conversationID, userID);

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

export async function getReadCursorsByConversationID(
  conversationID: string
): Promise<string> {
  try {
    const db = await getInstance();
    const execResult = databaseGetReadCursorsByConversationID(
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

export async function updateReadCursor(
  conversationID: string,
  userID: string,
  maxReadSeq: number
): Promise<string> {
  try {
    const db = await getInstance();
    databaseUpdateReadCursor(db, conversationID, userID, maxReadSeq);
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

export async function deleteReadCursor(
  conversationID: string,
  userID: string
): Promise<string> {
  try {
    const db = await getInstance();
    databaseDeleteReadCursor(db, conversationID, userID);
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

export async function deleteReadCursorsByConversationID(
  conversationID: string
): Promise<string> {
  try {
    const db = await getInstance();
    databaseDeleteReadCursorsByConversationID(db, conversationID);
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

export async function upsertReadCursor(cursorJSON: string): Promise<string> {
  try {
    const db = await getInstance();
    databaseUpsertReadCursor(db, cursorJSON);
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

export async function getAllReadSeqFromCursors(
  conversationID: string,
  excludeUserID: string
): Promise<string> {
  try {
    const db = await getInstance();
    const execResult = databaseGetAllReadSeqFromCursors(
      db,
      conversationID,
      excludeUserID
    );

    if (execResult.length === 0 || !execResult[0].values.length) {
      return formatResponse(0);
    }

    const allSeq = execResult[0].values[0][0] as number;
    return formatResponse(allSeq || 0);
  } catch (e) {
    console.error(e);
    return formatResponse(
      undefined,
      DatabaseErrorCode.ErrorInit,
      JSON.stringify(e)
    );
  }
}

export async function getReadStateDB(conversationID: string): Promise<string> {
  try {
    const db = await getInstance();
    const execResult = databaseGetReadState(db, conversationID);

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

export async function upsertReadStateDB(stateJSON: string): Promise<string> {
  try {
    const db = await getInstance();
    databaseUpsertReadState(db, stateJSON);
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

export async function updateReadStateAllReadSeqDB(
  conversationID: string,
  allReadSeq: number
): Promise<string> {
  try {
    const db = await getInstance();
    databaseUpdateReadStateAllReadSeq(db, conversationID, allReadSeq);
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

export async function deleteReadStateDB(
  conversationID: string
): Promise<string> {
  try {
    const db = await getInstance();
    databaseDeleteReadState(db, conversationID);
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
