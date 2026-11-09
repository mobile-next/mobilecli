export interface UIElement {
	type: string;
	label?: string;
	name?: string;
	value?: string;
	text?: string;
	identifier?: string;
	rect: {
		x: number;
		y: number;
		width: number;
		height: number;
	},
	// both platforms return a nested tree
	children?: UIElement[];
}

export interface Point {
	x: number;
	y: number;
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

// one entry of `apps list`. every field is reported on both platforms today; the
// shape assertions in shapes.ts are what keep that true.
export interface InstalledApp {
	packageName: string;
	appName: string;
	version: string;
	versionCode: string;
}

export interface AppsListResponse {
	status: string;
	data: InstalledApp[];
}

// `apps install` reads the metadata out of the artifact, so it has no appName
export interface InstallResult {
	message: string;
	app: {
		packageName: string;
		version: string;
		versionCode: string;
	};
}

export interface AppsInstallResponse {
	status: string;
	data: InstallResult;
}

export interface UninstallResult {
	packageName: string;
}

export interface AppsUninstallResponse {
	status: string;
	data: UninstallResult;
}
