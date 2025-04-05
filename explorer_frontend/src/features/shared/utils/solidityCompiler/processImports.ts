/**
 * Resolve a relative path to an absolute path
 * @param basePath - The base path to resolve the relative path from
 * @param relativePath - The relative path to resolve
 * @returns The absolute path
 */
function resolvePath(basePath: string, relativePath: string): string {
  const baseDir = basePath.split("/").slice(0, -1).join("/");

  // Handle different relative path cases
  if (relativePath.startsWith("./")) {
    return `${baseDir}/${relativePath.slice(2)}`;
  }
  if (relativePath.startsWith("../")) {
    const baseParts = baseDir.split("/");
    const relativeParts = relativePath.split("../");
    const stepsBack = relativeParts.length - 1;
    return `${baseParts.slice(0, -stepsBack).join("/")}/${relativeParts[relativeParts.length - 1]}`;
  }
  if (!relativePath.startsWith("@") && !relativePath.startsWith("/")) {
    return `${baseDir}/${relativePath}`;
  }
  return relativePath;
}

/**
 * Fetch contract content from unpkg
 * @param importPath - The import path to fetch
 * @returns The contract content or null if the fetch fails
 */
async function fetchContractContent(importPath: string): Promise<string | null> {
  try {
    const response = await fetch(`https://unpkg.com/${importPath}`);
    if (response.ok) {
      return await response.text();
    }
    return null;
  } catch (error) {
    console.error(`Failed to fetch ${importPath}:`, error);
    return null;
  }
}

/**
 * Process imports in a contract body
 * @param contractBody - The contract body to process
 * @param basePath - The base path to resolve relative paths
 * @param sources - The sources to store the processed imports
 * @param processedImports - The set of processed imports
 */
async function processImports(
  contractBody: string,
  basePath = "",
  sources: Record<string, { content: string }> = {},
  processedImports: Set<string> = new Set(),
): Promise<void> {
  const importRegex = /import\s+(?:{[^}]+}\s+from\s+)?["']([^"']+)["']/g;
  const imports = [...contractBody.matchAll(importRegex)].map((match) => match[1]);

  for (const importPath of imports) {
    // Resolve relative paths to absolute paths
    const absolutePath = importPath.startsWith(".")
      ? resolvePath(basePath, importPath)
      : importPath;

    if (processedImports.has(absolutePath)) continue;
    processedImports.add(absolutePath);

    if (sources[absolutePath]) continue;

    const content = await fetchContractContent(absolutePath);
    if (content) {
      let updatedContent = content;
      const nestedImports = [...content.matchAll(importRegex)].map((match) => match[1]);
      console.log("nestedImports", nestedImports);
      for (const nestedImport of nestedImports) {
        if (nestedImport.startsWith(".")) {
          const absoluteNestedPath = resolvePath(absolutePath, nestedImport);
          updatedContent = updatedContent.replace(
            new RegExp(`import\\s+["']${nestedImport}["']`, "g"),
            `import "${absoluteNestedPath}"`,
          );
        }
      }

      // Add to sources
      sources[absolutePath] = { content: updatedContent };

      // Recursively process nested imports
      await processImports(updatedContent, absolutePath, sources, processedImports);
    }
  }
}

export { processImports };
