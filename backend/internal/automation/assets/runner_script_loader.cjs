const path = require('path');
const { pathToFileURL } = require('url');

async function loadScriptModule(scriptPath) {
  const resolvedPath = path.resolve(String(scriptPath || ''));
  if (!resolvedPath) {
    throw new Error('scriptPath is required');
  }

  let requiredModule = null;
  let requireError = null;
  try {
    requiredModule = require(resolvedPath);
  } catch (error) {
    requireError = error;
  }

  const imported = async () => {
    const moduleURL = pathToFileURL(resolvedPath).href;
    return await import(`${moduleURL}?t=${Date.now()}`);
  };

  if (requiredModule && typeof requiredModule.run === 'function') {
    return requiredModule;
  }
  if (typeof requiredModule === 'function') {
    return { run: requiredModule };
  }
  if (requiredModule && requiredModule.default && typeof requiredModule.default.run === 'function') {
    return requiredModule.default;
  }

  try {
    const importedModule = await imported();
    if (importedModule && typeof importedModule.run === 'function') {
      return importedModule;
    }
    if (importedModule && typeof importedModule.default === 'function') {
      return { run: importedModule.default };
    }
    if (
      importedModule &&
      importedModule.default &&
      typeof importedModule.default.run === 'function'
    ) {
      return importedModule.default;
    }
  } catch (importError) {
    if (requireError) {
      throw requireError;
    }
    throw importError;
  }

  if (requireError) {
    throw requireError;
  }
  throw new Error('script must export run()');
}

module.exports = { loadScriptModule };