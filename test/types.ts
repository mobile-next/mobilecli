export interface UIElement {
	type: string;
	label?: string;
	name?: string;
	value?: string;
	identifier?: string;
	rect: {
		x: number;
		y: number;
		width: number;
		height: number;
	},
}

export interface UIDumpResponse {
	status: string;
	data: {
		elements: UIElement[];
	};
}

export interface DeviceInfoResponse {
	status: string;
	data: {
		device: {
			id: string;
			name: string;
			platform: string;
			type: string;
			version: string;
			state: string;
			screenSize: {
				width: number;
				height: number;
				scale: number;
			};
		};
	};
}

export interface ForegroundAppResponse {
	status: string;
	data: {
		packageName: string;
		appName: string;
		version: string;
	};
}
