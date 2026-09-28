import fs from 'fs';
import dotenv from 'dotenv';
dotenv.config();
const envFile = 'src/environments/environment.development.ts';
const prodFile = 'src/environments/environment.ts';
const content = `export const environment = {
  production: false,
  API_URL: "${process.env.API_URL || 'http://localhost:8080'}",
  isDebug: ${process.env.IS_DEBUG === 'true'}
};`;
const prodContent = `export const environment = {
  production: true,
  API_URL: "${process.env.API_URL || 'https://findvibefiber.onrender.com'}",
  isDebug: false
};`;
try {
  fs.writeFileSync(envFile, content);
  fs.writeFileSync(prodFile, prodContent);
} catch (err) {
  process.exit(1);
}
