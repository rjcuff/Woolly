export type OS = 'mac' | 'windows' | 'linux';

const latest = 'https://github.com/rjcuff/woolly/releases/latest/download';

export const macApp = `${latest}/Woolly-mac.dmg`;
export const windowsApp = `${latest}/Woolly-windows.exe`;

/** Guesses the visitor's operating system from the browser. Defaults to Mac. */
export function detectOS(userAgent: string = typeof navigator === 'undefined' ? '' : navigator.userAgent): OS {
  const ua = userAgent.toLowerCase();
  if (ua.includes('windows')) return 'windows';
  if (ua.includes('linux') && !ua.includes('android')) return 'linux';
  return 'mac';
}
