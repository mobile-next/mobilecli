#!/usr/bin/env node

const fs = require("fs");
const path = require("path");

const version = process.argv[2];
if (!version) {
	console.error("usage: update-versions.js <version>");
	process.exit(1);
}

const pkgPath = path.join(__dirname, "npm", "package.json");
const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
Object.keys(pkg.optionalDependencies).forEach(k => pkg.optionalDependencies[k] = version);
fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, "\t") + "\n");
console.log(`updated optionalDependencies in ${pkgPath} to ${version}`);
