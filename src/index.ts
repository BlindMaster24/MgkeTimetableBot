import { config } from '../config';
import { App } from './app';
import { validateConfig } from './config/validate';
import { startVanishCronJob as setupVanishCron } from './db/clean';

validateConfig(config);

const app = new App(config.services);

app.runServices();

setupVanishCron(app);

export { app };
