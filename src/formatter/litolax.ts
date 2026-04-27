import { GroupLessonExplain, TeacherLessonExplain } from '../services/parser/types';
import { GroupLessonOptions, ScheduleFormatter } from './abstract';

export class LitolaxScheduleFormatter extends ScheduleFormatter {
    public static readonly label: string = '💩 LitolaxStyle';

    protected formatGroupLesson(lesson: GroupLessonExplain, options: GroupLessonOptions): string {
        const line: string[] = [];

        if (lesson.subgroup) {
            line.push(this.Subgroup(`${lesson.subgroup}`));
        }

        line.push(this.Lesson(this.getLessonAlias(lesson.lesson)));

        if (lesson.type) {
            line.push(this.Type(lesson.type));
        }

        if (lesson.teacher) {
            line.push(this.Teacher(lesson.teacher));
        }

        if (lesson.comment != null) {
            line.push(this.Comment(lesson.comment));
        }

        return line.join(' ');
    }

    protected formatTeacherLesson(lesson: TeacherLessonExplain): string {
        const line: string[] = [];

        if (lesson.subgroup != null) {
            line.push(this.Subgroup(`${lesson.subgroup}`));
        }

        line.push(`${this.Group(lesson.group)}${this.Lesson(this.getLessonAlias(lesson.lesson))}`);

        if (lesson.type) {
            line.push(this.Type(lesson.type));
        }

        if (lesson.comment) {
            line.push(this.Comment(lesson.comment));
        }

        return line.join(' ');
    }

    protected afterGroupLessonFormat(lessons: GroupLessonExplain[]): string {
        const cabinets = lessons.map((lesson) => {
            return lesson.cabinet || '-';
        });

        return `Каб: ${cabinets.join(' ')}`;
    }

    protected afterTeacherLessonFormat(lesson: TeacherLessonExplain): string {
        return `Каб: ${lesson.cabinet || '-'}`;
    }

    protected formatLessonHeader(header: string, mainLessons: string, withSubgroups: boolean): string {
        const line: string[] = [header];

        if (!withSubgroups && mainLessons) {
            line.push(mainLessons);
        }

        return line.join('\n');
    }

    protected formatSubgroupLesson(value: string, currentIndex: number, lastIndex: number): string {
        return value;
    }

    protected GroupHeader(group: string): string {
        return `Группа: ${this.b(group)}`;
    }

    protected TeacherHeader(teacher: string): string {
        return `Преподаватель: ${this.b(this.getFullTeacherName(teacher))}`;
    }

    protected DayHeader(day: string, weekday: string): string {
        const hint: string | undefined = this.dayHint(day);

        return `\nДень - ${weekday + (hint ? ` ${this.i(hint)}` : '')}, ${day}\n`;
    }

    protected LessonHeader(i: number): string {
        return '\n' + this.b(`Пара: №${+i + 1}`);
    }

    protected Subgroup(subgroup: string): string {
        return `${subgroup}.`;
    }

    protected Lesson(lesson: string): string {
        return lesson;
    }

    protected Type(type: string): string {
        return `(${type})`;
    }

    protected Group(group: string): string {
        return `${group}-`;
    }

    protected Teacher(teacher: string): string {
        return teacher;
    }

    protected Cabinet(cabinet: string): string {
        return `Каб: ${cabinet}`;
    }

    protected Comment(comment: string): string {
        return `// ${comment}`;
    }

    protected NoLessons(): string {
        return this.i('Пар нет');
    }

    protected NoTimetable(): string {
        return 'Нет расписания для отображения';
    }
}
