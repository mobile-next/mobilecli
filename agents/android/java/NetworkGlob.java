package com.mobilenext.mobilecli;

import java.util.regex.Pattern;

/** Converts CLI URL globs to full-string matchers. Independent of Android. */
final class NetworkGlob {
	private NetworkGlob() { }

	static Pattern compile(String glob) {
		StringBuilder regex = new StringBuilder();
		for (int i = 0; i < glob.length(); i++) {
			char c = glob.charAt(i);
			if (c == '*') {
				if (i + 1 < glob.length() && glob.charAt(i + 1) == '*') { regex.append(".*"); i++; }
				else regex.append("[^/]*");
			} else if (c == '?') regex.append('.');
			else {
				if ("\\.[]{}()+-^$|".indexOf(c) >= 0) regex.append('\\');
				regex.append(c);
			}
		}
		return Pattern.compile(regex.toString(), Pattern.DOTALL);
	}
}
