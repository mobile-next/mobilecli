export interface JSONRPCRequest {
  jsonrpc: string;
  method: string;
  params?: any;
  id: number | string;
}

export interface JSONRPCResponse {
  jsonrpc: string;
  result?: any;
  error?: {
    code: number;
    message: string;
    data?: any;
  };
  id: number | string | null;
}

export const ErrCodeParseError = -32700;
export const ErrCodeInvalidRequest = -32600;
export const ErrCodeMethodNotFound = -32601;
export const ErrCodeServerError = -32000;