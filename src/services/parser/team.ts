import { AbstractParser } from "./abstract";
import { Team } from "./types";

export default class TeamParser extends AbstractParser {
    protected team: Team = {};

    protected get content(): Element {
        const content = this.document.querySelector('.entry.employees-list')
            || this.document.querySelector('.main.container #main-p')
            || this.document.querySelector('.common-page-left-block')
            || this.document.body;
        if (!content) {
            throw new Error('cannot get page content');
        }
        return content;
    }

    public run(team?: Team): Team {
        if (team) {
            this.team = team;
        }

        const cards: HTMLDivElement[] = Array.from(
            this.document.querySelectorAll('.entry.employees-list .employee-card')
        );
        if (cards.length > 0) {
            for (const card of cards) {
                this.parseEmployeeCard(card);
            }
            return this.team;
        }

        const items: HTMLDivElement[] = Array.from(
            this.document.querySelectorAll('.main.container #main-p > div.item')
        );
        for (const item of items) {
            this.parseItem(item);
        }

        return this.team;
    }

    private parseItem(item: HTMLDivElement) {
        // const img: HTMLImageElement | null = item.querySelector('div.preview > img');
        const content: HTMLDivElement | null = item.querySelector('.content');
        if (!content) {
            throw new Error('??? ????? ?????? ?? ???????');
        }

        const fullName = content.querySelector('h3')?.textContent;
        if (!fullName) {
            throw new Error('?????????? ???????? ?????? ??? ???????');
        }

        const shortName = fullName.match(/(\W+)\s(\W)\W+\s(\W)\W+/i)?.slice(1, 4)
            .map((part, i) => {
                if (i === 0) return part;
                return part + '.';
            }).join(' ');

        if (!shortName) {
            throw new Error('?????????? ????????????? ?????? ??? ? ???????????');
        }

        this.team[shortName] = fullName;
    }

    private parseEmployeeCard(card: HTMLDivElement) {
        const fullName = card.querySelector('h5.employee-card-title')?.textContent?.trim();
        if (!fullName) {
            return;
        }

        const shortName = fullName.match(/(\W+)\s(\W)\W+\s(\W)\W+/i)?.slice(1, 4)
            .map((part, i) => {
                if (i === 0) return part;
                return part + '.';
            }).join(' ');

        if (!shortName) {
            throw new Error('?????????? ????????????? ?????? ??? ? ???????????');
        }

        this.team[shortName] = fullName;
    }
}
