import { config } from "../config";
import { App } from "./app";
import { startVanishCronJob as setupVanishCron } from "./db/clean";

const app = new App(config.services);

app.runServices();

setupVanishCron(app);

export { app };