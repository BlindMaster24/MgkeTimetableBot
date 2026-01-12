import { Options, Sequelize } from 'sequelize';
import { config } from '../../config';

export const sequelize = new Sequelize(Object.assign<Options, Options>({
    logging: config.dev ? console.log : false,
    retry: {
        max: 3,
        match: [/SQLITE_BUSY/]
    },
    dialectOptions: {
        timeout: 5000
    }
}, config.db));

export * from './clean';
