package com.mobilenext.mobilecli;

class RpcException extends Exception {
	static final int PARSE_ERROR      = -32700;
	static final int INVALID_REQUEST  = -32600;
	static final int METHOD_NOT_FOUND = -32601;
	static final int INVALID_PARAMS   = -32602;
	static final int INTERNAL_ERROR   = -32603;
	static final int SERVER_ERROR     = -32000;
	static final int WEBVIEW_NOT_FOUND = -32100;

	final int code;

	RpcException(int code, String message) {
		super(message);
		this.code = code;
	}
}
