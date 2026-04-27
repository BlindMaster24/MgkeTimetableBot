import { GroupLessonExplain, TeacherLessonExplain } from '../services/parser/types';
import { GroupLessonOptions, ScheduleFormatter } from './abstract';

export class DefaultScheduleFormatter extends ScheduleFormatter {
    public static readonly label: string = '📝 Стуктурированный';

    protected formatGroupLesson(lesson: GroupLessonExplain, options: GroupLessonOptions): string {
        const line: string[] = [];

        if (options.showSubgroup && lesson.subgroup != null) {
            line.push(this.Subgroup(`${lesson.subgroup}`));
        }

        if (options.showLesson) {
            line.push(this.Lesson(this.getLessonAlias(lesson.lesson)));
        }

        if (options.showType && lesson.type) {
            line.push(this.Type(lesson.type));
        }

        if (options.showTeacher && lesson.teacher) {
            line.push(this.Teacher(lesson.teacher));
        }

        if (options.showCabinet && lesson.cabinet != null) {
            line.push(this.Cabinet(lesson.cabinet));
        }

        if (options.showComment && lesson.comment != null) {
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

        if (lesson.cabinet != null) {
            line.push(this.Cabinet(lesson.cabinet));
        }

        if (lesson.comment) {
            line.push(this.Comment(lesson.comment));
        }

        return line.join(' ');
    }

    protected formatLessonHeader(header: string, mainLessons: string, withSubgroups: boolean): string {
        const line: string[] = [header];

        if (mainLessons) {
            line.push(mainLessons);
        }

        return line.join('');
    }

    protected formatSubgroupLesson(value: string, currentIndex: number, lastIndex: number): string {
        if (currentIndex === lastIndex) {
            value = '└── ' + value;
        } else {
            value = '├── ' + value;
        }

        return value;
    }

    protected GroupHeader(group: string): string {
        return `- Группа '${group}' -`;
    }

    protected TeacherHeader(teacher: string): string {
        return `- Преподаватель '${this.getFullTeacherName(teacher)}' -`;
    }

    protected DayHeader(day: string, weekday: string): string {
        const hint: string | undefined = this.dayHint(day);

        return `__ ${this.b(weekday + (hint ? ` ${this.i(hint)}` : ''))}, ${day} __`;
    }

    protected LessonHeader(i: number): string {
        return `${+i + 1}. `;
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
        return `{${cabinet}}`;
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
