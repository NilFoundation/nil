#!/usr/bin/env node
const fs = require('fs');
const path = require('path');

// Get the project directory
const projectDir = process.argv[2] || 'new-nil-project';
const projectPath = path.resolve(projectDir);
const templatePath = path.join(__dirname, '..');

console.log(`Creating a new project in ${projectDir}...`);

// Create the directory for the new project
fs.mkdirSync(projectPath, { recursive: true });

function createPackageJson(templateDir, projectDir) {
  const templatePackageJsonPath = path.join(templateDir, 'package.json');
  const projectPackageJsonPath = path.join(projectDir, 'package.json');

  // Read and parse the template package.json
  const packageJsonContent = JSON.parse(fs.readFileSync(templatePackageJsonPath, 'utf-8'));

  // Modify package.json fields
  packageJsonContent.name = path.basename(projectDir); // Set project name as the directory name
  packageJsonContent.version = "1.0.0"; // Reset version to 1.0.0

  // Remove template scaffolding fields
  delete packageJsonContent.bin;
  delete packageJsonContent.scripts.start;

  // Write the modified package.json to the new project directory
  fs.writeFileSync(
    projectPackageJsonPath,
    JSON.stringify(packageJsonContent, null, 2)
  );

  console.log('Created package.json');
}

function copyFile(srcFilePath, destFilePath) {
    const fullSrcPath = path.join(templatePath, srcFilePath);
    const fullDestPath = path.join(projectPath, destFilePath);

    // Ensure the destination directory exists
    fs.mkdirSync(path.dirname(fullDestPath), { recursive: true });

    // Copy the file if it exists
    if (fs.existsSync(fullSrcPath)) {
        fs.copyFileSync(fullSrcPath, fullDestPath);
    } else {
        console.warn(`File not found: ${srcFilePath}`);
    }
}

createPackageJson(templatePath, projectPath);
copyFile('.env.example', '.env');
copyFile('ignition/modules/Incrementer.ts', 'ignition/modules/Incrementer.ts');
copyFile('contracts/Incrementer.sol', 'contracts/Incrementer.sol');

console.log('Project setup complete!');
