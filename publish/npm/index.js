#!/usr/bin/env node

const { execSync } = require("node:child_process");

execSync(__dirname + "/bin/mobilectl", {stdio: "inherit"});
