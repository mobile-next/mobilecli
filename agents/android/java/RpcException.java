package com.mobilenext.mobilecli;

class RpcException extends Exception {
	final int code;

	RpcException(int code, String message) {
		super(message);
		this.code = code;
	}
}
