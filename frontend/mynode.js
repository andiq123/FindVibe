import fs from 'fs';
import dotenv from 'dotenv';
dotenv.config();
const prodFile = 'src/environments/environment.ts';
const prodContent = `export const environment = {
  production: true,
  API_URL: ${JSON.stringify(process.env.API_URL || 'https://find-vibe.firewifi.online')},
  isDebug: false
};`;
try {
  fs.writeFileSync(prodFile, prodContent);
} catch (err) {
  process.exit(1);
}
