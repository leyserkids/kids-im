/**
 * WASM 本地开发同步脚本
 *
 * 用于将编译产物同步到前端项目的 node_modules 目录
 *
 * 使用方法:
 *   npm run sync           # 只复制文件（不编译）
 *   npm run sync -- --wasm # 编译 WASM 后复制
 *   npm run sync -- --js   # 编译 JS SDK 后复制
 *   npm run sync -- --all  # 全部编译后复制
 *
 * 环境变量:
 *   PROJECT_PATH_KIDS_IM_FRONTEND - 前端项目路径（必需）
 */

import { spawn } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// ============================================================================
// 常量配置
// ============================================================================

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT_DIR = path.resolve(__dirname, '../..');
const SDK_JS_WASM_DIR = path.resolve(ROOT_DIR, 'sdk-js-wasm');
const SDK_CORE_DIR = path.resolve(ROOT_DIR, 'sdk-core');

const TARGET_PACKAGE = '@openim/wasm-client-sdk';
const ENV_VAR_NAME = 'PROJECT_PATH_KIDS_IM_FRONTEND';

// 源文件路径
const PATHS = {
  wasmOutput: path.join(SDK_CORE_DIR, '_output/bin/openIM.wasm'),
  versionFile: path.join(SDK_CORE_DIR, 'version/version'),
  libDir: path.join(SDK_JS_WASM_DIR, 'lib'),
};

// ============================================================================
// 工具函数
// ============================================================================

const color = (code) => (text) => `\x1b[${code}m${text}\x1b[0m`;
const colors = {
  red: color(31),
  green: color(32),
  yellow: color(33),
  cyan: color(36),
  gray: color(90),
  bold: color(1),
};

const log = {
  success: (msg) => console.log(colors.green('✓'), msg),
  warn: (msg) => console.log(colors.yellow('⚠'), msg),
  error: (msg) => console.log(colors.red('✗'), msg),
  step: (msg) => console.log(colors.cyan(`\n[${msg}]`)),
  cmd: (msg) => console.log(colors.gray(`  执行: ${msg}`)),
};

function expandHome(filepath) {
  if (!filepath) return filepath;
  return filepath.startsWith('~/') ? path.join(os.homedir(), filepath.slice(2)) : filepath;
}

function formatSize(bytes) {
  const units = ['B', 'KB', 'MB'];
  let i = 0;
  while (bytes >= 1024 && i < units.length - 1) {
    bytes /= 1024;
    i++;
  }
  return i === 0 ? `${bytes} ${units[i]}` : `${bytes.toFixed(1)} ${units[i]}`;
}

function countFiles(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).reduce((count, entry) => {
    return count + (entry.isFile() ? 1 : countFiles(path.join(dir, entry.name)));
  }, 0);
}

function ensureDir(dir) {
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
}

// ============================================================================
// 命令执行
// ============================================================================

function exec(command, args, options = {}) {
  return new Promise((resolve, reject) => {
    const proc = spawn(command, args, {
      stdio: 'inherit',
      shell: process.platform === 'win32',
      ...options,
    });
    proc.on('close', (code) => (code === 0 ? resolve() : reject(new Error(`命令退出码: ${code}`))));
    proc.on('error', reject);
  });
}

// ============================================================================
// 编译任务
// ============================================================================

async function buildWasm() {
  log.step('编译 WASM');
  ensureDir(path.dirname(PATHS.wasmOutput));

  const ldflags = '-s -w';
  log.cmd(`GOOS=js GOARCH=wasm go build -trimpath -ldflags "${ldflags}" -o _output/bin/openIM.wasm wasm/cmd/main.go`);

  await exec('go', ['build', '-trimpath', '-ldflags', ldflags, '-o', '_output/bin/openIM.wasm', 'wasm/cmd/main.go'], {
    cwd: SDK_CORE_DIR,
    env: { ...process.env, GOOS: 'js', GOARCH: 'wasm' },
  });
  log.success('完成');
}

async function buildJs() {
  log.step('编译 JS SDK');
  log.cmd('npm run build');
  await exec('npm', ['run', 'build'], { cwd: SDK_JS_WASM_DIR });
  log.success('完成');
}

// ============================================================================
// 文件复制
// ============================================================================

function copyFile(src, dest, label, extraInfo) {
  if (!fs.existsSync(src)) {
    log.warn(`${label} 不存在，跳过`);
    return false;
  }
  fs.copyFileSync(src, dest);
  const info = typeof extraInfo === 'function' ? extraInfo() : formatSize(fs.statSync(dest).size);
  log.success(`${label} (${info})`);
  return true;
}

function copyDir(src, dest, label) {
  if (!fs.existsSync(src)) {
    log.warn(`${label} 不存在，跳过（使用 --js 编译）`);
    return false;
  }
  if (fs.existsSync(dest)) {
    fs.rmSync(dest, { recursive: true });
  }
  fs.cpSync(src, dest, { recursive: true });
  log.success(`${label} (${countFiles(dest)} 文件)`);
  return true;
}

async function copyToFrontend(targetDir) {
  log.step('复制到前端');
  console.log(colors.gray(`  目标: ${targetDir}`));

  const assetsDir = path.join(targetDir, 'assets');
  const libDir = path.join(targetDir, 'lib');
  ensureDir(assetsDir);

  copyFile(PATHS.wasmOutput, path.join(assetsDir, 'openIM.wasm'), 'openIM.wasm');
  copyFile(PATHS.versionFile, path.join(assetsDir, 'version'), 'version', () =>
    fs.readFileSync(PATHS.versionFile, 'utf-8').trim()
  );
  copyDir(PATHS.libDir, libDir, 'lib/');
}

// ============================================================================
// 参数解析与帮助
// ============================================================================

function parseArgs() {
  const args = process.argv.slice(2);
  const options = { buildWasm: false, buildJs: false, help: false };

  for (const arg of args) {
    switch (arg) {
      case '--wasm':
        options.buildWasm = true;
        break;
      case '--js':
        options.buildJs = true;
        break;
      case '--all':
        options.buildWasm = options.buildJs = true;
        break;
      case '--help':
      case '-h':
        options.help = true;
        break;
      default:
        log.warn(`未知参数: ${arg}`);
    }
  }
  return options;
}

function showHelp() {
  console.log(`
${colors.bold('WASM 本地开发同步脚本')}

${colors.cyan('使用方法:')}
  npm run sync           只复制文件（不编译）
  npm run sync -- --wasm 编译 WASM 后复制
  npm run sync -- --js   编译 JS SDK 后复制
  npm run sync -- --all  全部编译后复制

${colors.cyan('选项:')}
  --wasm      先编译 sdk-core WASM，再复制
  --js        先编译 sdk-js-wasm，再复制
  --all       编译 WASM + JS，再复制
  --help, -h  显示此帮助信息

${colors.cyan('环境变量:')}
  ${ENV_VAR_NAME}  前端项目路径（${colors.yellow('必需')}）

${colors.cyan('示例:')}
  export ${ENV_VAR_NAME}=/path/to/frontend
  npm run sync -- --all
`);
}

// ============================================================================
// 主函数
// ============================================================================

function validatePaths(frontendPath) {
  if (!frontendPath) {
    log.error(`未设置环境变量 ${ENV_VAR_NAME}`);
    console.log(colors.gray(`\n  请设置前端项目路径:`));
    console.log(colors.gray(`  export ${ENV_VAR_NAME}=/path/to/frontend\n`));
    process.exit(1);
  }

  const resolved = expandHome(frontendPath);
  if (!fs.existsSync(resolved)) {
    log.error(`前端项目路径不存在: ${resolved}`);
    process.exit(1);
  }

  const targetDir = path.join(resolved, '../..', 'node_modules', TARGET_PACKAGE);
  if (!fs.existsSync(targetDir)) {
    log.error(`目标包不存在: ${targetDir}`);
    console.log(colors.gray(`  请确保前端项目已安装 ${TARGET_PACKAGE}\n`));
    process.exit(1);
  }

  return targetDir;
}

async function main() {
  const options = parseArgs();

  if (options.help) {
    showHelp();
    process.exit(0);
  }

  console.log(colors.bold('\n=== WASM 本地开发同步 ==='));

  const targetDir = validatePaths(process.env[ENV_VAR_NAME]);

  try {
    if (options.buildWasm) await buildWasm();
    if (options.buildJs) await buildJs();
    await copyToFrontend(targetDir);
    console.log(colors.green('\n✓ 全部完成!\n'));
  } catch (err) {
    log.error(err.message);
    process.exit(1);
  }
}

main();
